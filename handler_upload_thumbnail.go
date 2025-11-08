package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here

	const maxMemory = 10 << 20

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse form", err)
		return
	}

	media, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't extract media", err)
		return
	}

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "Malformed request header", nil)
		return
	}

	allowedTypes := map[string]int{
		"image/jpg": 0,
		"image/png": 0,
	}
	if _, ok := allowedTypes[mediaType]; !ok {
		respondWithError(w, http.StatusBadRequest, "Incorrect media type", nil)
		return
	}

	mediaType = strings.Split(mediaType, "/")[1]

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

	byteSlice := make([]byte, 32)
	_, err = rand.Read(byteSlice)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating byte slice", err)
	}
	randomString := base64.RawURLEncoding.EncodeToString(byteSlice)
	thumbnailFilename := fmt.Sprintf("%s.%s", randomString, mediaType)
	thumbnailFilepath := filepath.Join(cfg.assetsRoot, thumbnailFilename)

	file, err := os.Create(thumbnailFilepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving thumbnail", err)
		return
	}
	defer file.Close()

	_, err = io.Copy(file, media)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving thumbnail", err)
		return
	}

	dataUrl := fmt.Sprintf("http://localhost:8091/assets/%s", thumbnailFilename)
	video.ThumbnailURL = &dataUrl
	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, struct{}{})
}
