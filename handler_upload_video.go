package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(
			w,
			http.StatusUnauthorized,
			"You are not authorised to access this video",
			err,
		)
		return
	}

	const maxMemory = 10 << 30

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse form", err)
		return
	}

	media, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't extract media", err)
		return
	}
	defer media.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Malformed request header", nil)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Incorrect media type", nil)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely_upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create temp file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, media)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to copy data to temp file", err)
		return
	}

	tempFile.Seek(0, io.SeekStart)

	s3Bucket := "tubely-37846263"
	mimeType := "video/mp4"

	byteSlice := make([]byte, 32)
	_, err = rand.Read(byteSlice)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating byte slice", err)
		return
	}
	randomString := base64.RawURLEncoding.EncodeToString(byteSlice)
	videoFilename := fmt.Sprintf("%s.mp4", randomString)
	clientParams := s3.PutObjectInput{
		Bucket:      &s3Bucket,
		Key:         &videoFilename,
		Body:        tempFile,
		ContentType: &mimeType,
	}
	_, err = cfg.s3Client.PutObject(r.Context(), &clientParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading to aws", err)
		return
	}

	videoUrl := fmt.Sprintf("https://%s.s3.eu-north-1.amazonaws.com/%s", s3Bucket, videoFilename)
	video.VideoURL = &videoUrl
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video data", err)
		return
	}
}
