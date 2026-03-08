package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type R2 struct {
	client *s3.Client
	bucket string
}

func NewR2() (*R2, error) {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucket := os.Getenv("R2_BUCKET")
	if bucket == "" {
		bucket = "gym-app"
	}
	if accountID == "" || accessKey == "" || secretKey == "" {
		return nil, nil
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
	return &R2{client: client, bucket: bucket}, nil
}

// PutPRImage uploads image bytes to pr/{userID}/{prID}.png
func (r *R2) PutPRImage(ctx context.Context, userID, prID uuid.UUID, body io.Reader) error {
	key := fmt.Sprintf("pr/%s/%s.png", userID.String(), prID.String())
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	return err
}

// PutPRImageBytes is a convenience for byte slices.
func (r *R2) PutPRImageBytes(ctx context.Context, userID, prID uuid.UUID, data []byte) error {
	return r.PutPRImage(ctx, userID, prID, bytes.NewReader(data))
}

// PresignPRImage returns a presigned GET URL for pr/{userID}/{prID}.png
func (r *R2) PresignPRImage(ctx context.Context, userID, prID uuid.UUID, expirySec int) (string, error) {
	key := fmt.Sprintf("pr/%s/%s.png", userID.String(), prID.String())
	presign := s3.NewPresignClient(r.client)
	result, err := presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(time.Duration(expirySec)*time.Second))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}
