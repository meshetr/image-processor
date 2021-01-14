package main

import (
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/spf13/viper"
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
		logger:        logger,
		db:            db,
		storageClient: storageClient,
	}
}

func (service imageService) ProcessImage(ctx context.Context, id uint32) error {
	level.Info(service.logger).Log("msg", "Received ID: "+fmt.Sprintf("%d", id))
	var photo Photo
	service.db.First(&photo, id)
	go service.resizeImage(photo, 1280, "url_large")
	go service.resizeImage(photo, 960, "url_medium")
	go service.resizeImage(photo, 640, "url_small")
	return nil
}

func (service imageService) resizeImage(photo Photo, pix int, fieldName string) {
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
		level.Error(service.logger).Log("API call failed", err)
		service.resizeImageFallback(photo, pix, fieldName)
		return
	}

	var res map[string]interface{}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&res)

	response, err := http.Get(fmt.Sprintf("%v", res["kraked_url"]))
	if err != nil {
		level.Error(service.logger).Log("API call failed", err)
		service.resizeImageFallback(photo, pix, fieldName)
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		level.Error(service.logger).Log("Received non 200 response code", response.StatusCode)
		service.resizeImageFallback(photo, pix, fieldName)
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
		level.Error(service.logger).Log("Storage upload failed", err)
		service.resizeImageFallback(photo, pix, fieldName)
		return
	}

	service.db.Model(&photo).Update(fieldName, url)
}

func (service imageService) resizeImageFallback(photo Photo, pix int, fieldName string) {
	viper.AutomaticEnv()
	response, err := http.Get(fmt.Sprintf("https://api.imageresizer.io/v1/images?key=%s&url=%s",
		viper.GetString("IMAGERESIZER_API_KEY"),
		url.QueryEscape(photo.UrlOriginal)))
	if err != nil {
		level.Error(service.logger).Log("API call failed", err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		level.Error(service.logger).Log("Received non 200 response code", response.StatusCode)
		return
	}
	var res map[string]interface{}
	json.NewDecoder(response.Body).Decode(&res)
	imageresizerResponse := res["response"].(map[string]interface{})
	id := imageresizerResponse["id"].(string)
	url := "https://im.ages.io/" + id + "?width=" + fmt.Sprint(pix)
	service.db.Model(&photo).Update(fieldName, url)
}
