package data

import (
	"time"

	"github.com/BitofferHub/pkg/middlewares/log"
	"gorm.io/gorm"
)

var MsgRecordNsp MsgRecord

type MsgRecord struct {
	ID           int64
	Subject      string
	To           string
	MsgId        string
	TemplateID   string
	TemplateData string
	Channel      int
	SourceID     string
	Status       int        // 添加状态字段
	RetryCount   int        // 重试次数，默认为0
	CreateTime   *time.Time `gorm:"column:create_time;default:null"`
	ModifyTime   *time.Time `gorm:"column:modify_time;default:null"`
}

// TableName 表名
func (p *MsgRecord) TableName() string {
	return "t_msg_record"
}

// Find 查找记录
func (p *MsgRecord) Find(db *gorm.DB, msgID string) (*MsgRecord, error) {
	var data = &MsgRecord{}
	err := db.Where("msg_id= ?", msgID).First(data).Error
	return data, err
}

// Create 创建记录
func (p *MsgRecord) Create(db *gorm.DB, dt *MsgRecord) error {
	data := dt
	err := db.Create(data).Error
	return err
}

// UpdateStatus 更新消息记录状态
func (p *MsgRecord) UpdateStatus(db *gorm.DB, msgID string, status int) error {
	err := db.Model(&MsgRecord{}).Where("msg_id = ?", msgID).Update("status", status).Error
	return err
}

// UpdateRetryCount 更新消息记录的重试次数
func (p *MsgRecord) UpdateRetryCount(db *gorm.DB, msgID string, retryCount int) error {
	err := db.Model(&MsgRecord{}).Where("msg_id = ?", msgID).Update("retry_count", retryCount).Error
	return err
}

// IncrementRetryCount 增加消息记录的重试次数
func (p *MsgRecord) IncrementRetryCount(db *gorm.DB, msgID string) (int, error) {
	// 先查询当前重试次数
	record, err := p.Find(db, msgID)
	if err != nil {
		return 0, err
	}

	newCount := record.RetryCount + 1
	log.Infof("消息 %s 当前重试次数将从 %d 增加到 %d", msgID, record.RetryCount, newCount)
	// 增加重试次数

	// 更新数据库
	err = db.Model(&MsgRecord{}).Where("msg_id = ?", msgID).Update("retry_count", newCount).Error
	if err != nil {
		return 0, err
	}

	return newCount, nil
}
