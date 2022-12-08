package services

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type VideoUpload struct {
	Paths        []string
	VideoPath    string
	OutputBucket string
	Errors       []string
}

func NewVideoUpload() *VideoUpload {
	return &VideoUpload{}
}

func (vu *VideoUpload) UploadObject(ObjectPath string, minioClient *minio.Client, ctx context.Context) error {

	path := strings.Split(ObjectPath, os.Getenv("localStoragePath")+"/")
	objectName := path[1]
	filePath := ObjectPath
	// contentType := "application/mp4"

	info, err := minioClient.FPutObject(ctx, vu.OutputBucket, objectName, filePath, minio.PutObjectOptions{}) //ContentType: contentType
	if err != nil {
		return err
	}

	log.Printf("Successfully uploaded %s of size %d\n", objectName, info.Size)
	return nil
}

func (vu *VideoUpload) loadPaths() error {
	err := filepath.Walk(vu.VideoPath, func(path string, info os.FileInfo, err error) error {

		if !info.IsDir() {
			vu.Paths = append(vu.Paths, path)
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (vu *VideoUpload) ProcessUpload(concurrency int, doneUpload chan string) error {
	in := make(chan int, runtime.NumCPU())
	returnChannel := make(chan string)

	err := vu.loadPaths()
	if err != nil {
		return err
	}

	uploadClient, ctx, err := getClientUpload()
	if err != nil {
		return err
	}

	for process := 0; process < concurrency; process++ {
		go vu.uploadWorker(in, returnChannel, uploadClient, ctx)
	}

	go func() {
		for x := 0; x < len(vu.Paths); x++ {
			in <- x
		}
		close(in)
	}()

	for r := range returnChannel {
		if r != "" {
			doneUpload <- r
			break
		}
	}
	// close(returnChannel)
	return nil
}

func (vu *VideoUpload) uploadWorker(in chan int, returnChan chan string, uploadClient *minio.Client, ctx context.Context) {
	for x := range in {
		err := vu.UploadObject(vu.Paths[x], uploadClient, ctx)
		if err != nil {
			vu.Errors = append(vu.Errors, vu.Paths[x])
			log.Printf("error during upload: %v. Error %v", vu.Paths[x], err)
			returnChan <- err.Error()
		}

		returnChan <- ""
	}

	returnChan <- "upload completed"
}

func getClientUpload() (*minio.Client, context.Context, error) {
	ctx := context.Background()
	endpoint := os.Getenv("bucketEndpoint")
	accessKeyID := os.Getenv("bucketAccessKeyID")
	secretAccessKey := os.Getenv("bucketSecretAccessKey")
	useSSL := true

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})

	if err != nil {
		return nil, nil, err
	}

	return minioClient, ctx, nil
}
