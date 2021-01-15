package main

import (
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/spf13/viper"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Service interface {
	ProcessImage(ctx context.Context, id uint32) error
}

type imageService struct {
	logger        log.Logger
	db            *gorm.DB
	storageClient *storage.Client
}

type Photo struct {
	IdPhoto     uint `gorm:"primaryKey"`
	IdAd        uint
	UrlOriginal string
	UrlSmall    string
	UrlMedium   string
	UrlLarge    string
}

func (Photo) TableName() string {
	return "t_photo"
}

func MakeService(logger log.Logger, db *gorm.DB, storageClient *storage.Client) Service {
	db.AutoMigrate(&Photo{})
	return &imageService{
		logger:        log.With(logger, "component", "service"),
		db:            db,
		storageClient: storageClient,
	}
}

func (service imageService) ProcessImage(ctx context.Context, id uint32) error {
	md, _ := metadata.FromIncomingContext(ctx)
	logger := log.With(service.logger, "request-id", md["request-id"][0])

	level.Info(logger).Log("msg", "request received", "context", fmt.Sprintf("\"id\":%d", id))
	var photo Photo
	service.db.First(&photo, id)
	go service.resizeImage(logger, photo, 1280, "url_large")
	go service.resizeImage(logger, photo, 960, "url_medium")
	go service.resizeImage(logger, photo, 640, "url_small")
	return nil
}

func (service imageService) resizeImage(logger log.Logger, photo Photo, pix int, fieldName string) {
	viper.AutomaticEnv()
	apiUrl := "https://api.kraken.io/v1/url"
	payload := strings.NewReader(`{
		"auth": {
			"api_key": "` + viper.GetString("KRAKEN_API_KEY") + `",
			"api_secret": "` + viper.GetString("KRAKEN_API_SECRET") + `"
		},
		"url": "` + photo.UrlOriginal + `",
		"resize": {
			"width": ` + fmt.Sprint(pix) + `,
			"height": ` + fmt.Sprint(pix) + `,
			"strategy": "auto",
			"enhance": true
		},
		"wait": true
	}`)

	resp, err := http.Post(apiUrl, "application/json", payload)

	if err != nil {
		level.Error(logger).Log("context", "kraken.io API call", "msg", err)
		service.resizeImageFallback(logger, photo, pix, fieldName)
		return
	}

	var res map[string]interface{}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&res)

	response, err := http.Get(fmt.Sprintf("%v", res["kraked_url"]))
	if err != nil {
		level.Error(logger).Log("context", "kraken.io image download", "msg", err)
		service.resizeImageFallback(logger, photo, pix, fieldName)
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		level.Error(logger).Log("context", "kraken.io image download", "msg", fmt.Sprintf("Received non 200 response code: %d", response.StatusCode))
		service.resizeImageFallback(logger, photo, pix, fieldName)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
	defer cancel()
	bucketName := "meshetr-images"
	objectName := fmt.Sprintf("%d-%d", photo.IdAd, time.Now().UnixNano())
	url := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectName)
	writer := service.storageClient.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	defer writer.Close()
	if _, err := io.Copy(writer, response.Body); err != nil {
		level.Error(logger).Log("context", "Storage upload", "msg", err)
		service.resizeImageFallback(logger, photo, pix, fieldName)
		return
	}

	service.db.Model(&photo).Update(fieldName, url)
	logContext, _ := json.Marshal(photo)
	level.Info(logger).Log("context", logContext, "msg", "Resized photo successfully uploaded.")
}

func (service imageService) resizeImageFallback(logger log.Logger, photo Photo, pix int, fieldName string) {
	logContext, _ := json.Marshal(photo)
	level.Warn(logger).Log("context", logContext, "msg", "Image resizing FALLBACK.")

	viper.AutomaticEnv()
	response, err := http.Get(fmt.Sprintf("https://api.imageresizer.io/v1/images?key=%s&url=%s",
		viper.GetString("IMAGERESIZER_API_KEY"),
		url.QueryEscape(photo.UrlOriginal)))
	if err != nil {
		level.Error(logger).Log("context", "imageresizer API call", "msg", err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		level.Error(logger).Log("context", "imageresizer API call", "msg", fmt.Sprintf("Received non 200 response code: %d", response.StatusCode))
		return
	}
	var res map[string]interface{}
	json.NewDecoder(response.Body).Decode(&res)
	imageresizerResponse := res["response"].(map[string]interface{})
	id := imageresizerResponse["id"].(string)
	url := "https://im.ages.io/" + id + "?width=" + fmt.Sprint(pix)

	service.db.Model(&photo).Update(fieldName, url)
	level.Info(logger).Log("context", logContext, "msg", "Resized photo successfully uploaded (FALLBACK).")
}
