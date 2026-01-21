package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pocketbase/pocketbase"
)

const (
	appName       = "crm"
	backupHour    = 3 // 3 AM AEST
	retentionDays = 30
)

// scheduleBackups runs daily backups at the specified hour (AEST)
func scheduleBackups(app *pocketbase.PocketBase) {
	// Wait for app to fully start
	time.Sleep(30 * time.Second)

	loc, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		log.Printf("[Backup] Warning: Could not load timezone, using UTC: %v", err)
		loc = time.UTC
	}

	for {
		now := time.Now().In(loc)
		next := time.Date(now.Year(), now.Month(), now.Day(), backupHour, 0, 0, 0, loc)
		if now.After(next) {
			next = next.Add(24 * time.Hour)
		}

		duration := time.Until(next)
		log.Printf("[Backup] Next backup scheduled for %s (in %v)", next.Format("2006-01-02 15:04 MST"), duration.Round(time.Minute))

		time.Sleep(duration)

		if err := runBackup(app); err != nil {
			log.Printf("[Backup] ERROR: %v", err)
		}
	}
}

// runBackup creates a PocketBase backup and uploads it to S3
func runBackup(app *pocketbase.PocketBase) error {
	log.Printf("[Backup] Starting daily backup...")

	backupName := fmt.Sprintf("%s-db-%s.zip", appName, time.Now().Format("2006-01-02"))

	// Create backup using PocketBase API
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := app.CreateBackup(ctx, backupName); err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	// Find the backup file
	dataDir := app.DataDir()
	backupPath := filepath.Join(dataDir, "backups", backupName)

	// Verify backup was created
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found at %s", backupPath)
	}

	// Upload to S3
	if err := uploadBackupToS3(backupPath, backupName); err != nil {
		return fmt.Errorf("upload to S3: %w", err)
	}

	// Delete local backup to save space
	if err := os.Remove(backupPath); err != nil {
		log.Printf("[Backup] Warning: Failed to delete local backup: %v", err)
	}

	// Clean up old backups from S3
	if err := cleanOldBackups(); err != nil {
		log.Printf("[Backup] Warning: Failed to clean old backups: %v", err)
	}

	log.Printf("[Backup] Completed successfully: %s", backupName)
	return nil
}

// uploadBackupToS3 uploads a backup file to the Tigris S3 bucket
func uploadBackupToS3(localPath, backupName string) error {
	bucket := os.Getenv("BACKUP_BUCKET_NAME")
	endpoint := os.Getenv("BACKUP_ENDPOINT_URL")
	accessKey := os.Getenv("BACKUP_ACCESS_KEY_ID")
	secretKey := os.Getenv("BACKUP_SECRET_ACCESS_KEY")

	if bucket == "" || accessKey == "" || secretKey == "" {
		return fmt.Errorf("backup S3 credentials not configured (BACKUP_BUCKET_NAME, BACKUP_ACCESS_KEY_ID, BACKUP_SECRET_ACCESS_KEY)")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open backup file: %w", err)
	}
	defer file.Close()

	key := fmt.Sprintf("%s/database/%s", appName, backupName)

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("upload to S3: %w", err)
	}

	log.Printf("[Backup] Uploaded to s3://%s/%s", bucket, key)
	return nil
}

// cleanOldBackups removes backups older than retentionDays from S3
func cleanOldBackups() error {
	bucket := os.Getenv("BACKUP_BUCKET_NAME")
	endpoint := os.Getenv("BACKUP_ENDPOINT_URL")
	accessKey := os.Getenv("BACKUP_ACCESS_KEY_ID")
	secretKey := os.Getenv("BACKUP_SECRET_ACCESS_KEY")

	if bucket == "" || accessKey == "" {
		return nil // Skip if not configured
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	// List objects in the app's backup folder
	prefix := fmt.Sprintf("%s/database/", appName)
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	var toDelete []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return fmt.Errorf("list objects: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.LastModified != nil && obj.LastModified.Before(cutoffDate) {
				toDelete = append(toDelete, *obj.Key)
			}
		}
	}

	// Delete old backups
	for _, key := range toDelete {
		_, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			log.Printf("[Backup] Warning: Failed to delete old backup %s: %v", key, err)
		} else {
			log.Printf("[Backup] Deleted old backup: %s", key)
		}
	}

	if len(toDelete) > 0 {
		log.Printf("[Backup] Cleaned up %d old backup(s)", len(toDelete))
	}

	return nil
}

// RunBackupNow can be called to trigger an immediate backup (useful for testing)
func RunBackupNow(app *pocketbase.PocketBase) error {
	return runBackup(app)
}
