package database

import (
	"context"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
)

// GetNotifyUrls retrieves all UI-sourced notification URLs from the database
func (sr *scrutinyRepository) GetNotifyUrls(ctx context.Context) ([]models.NotifyUrl, error) {
	var urls []models.NotifyUrl
	if err := sr.gormClient.WithContext(ctx).Find(&urls).Error; err != nil {
		return nil, err
	}
	return urls, nil
}

// SaveNotifyUrl creates a new UI-sourced notification URL
func (sr *scrutinyRepository) SaveNotifyUrl(ctx context.Context, notifyUrl *models.NotifyUrl) error {
	notifyUrl.Source = "ui"
	return sr.gormClient.WithContext(ctx).Create(notifyUrl).Error
}

// DeleteNotifyUrl removes a notification URL by ID
func (sr *scrutinyRepository) DeleteNotifyUrl(ctx context.Context, id uint) error {
	return sr.gormClient.WithContext(ctx).Delete(&models.NotifyUrl{}, id).Error
}
