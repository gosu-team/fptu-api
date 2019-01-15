package models

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gosu-team/fptu-api/config"
)

// Confession ...
type Confession struct {
	ID        int        `json:"id" gorm:"primary_key"`
	CreatedAt *time.Time `json:"created_at, omitempty"`
	UpdatedAt *time.Time `json:"updated_at, omitempty"`
	DeletedAt *time.Time `json:"deleted_at, omitempty" sql:"index"`

	Content  string `json:"content" gorm:"not null; type:text;"`
	Sender   string `json:"sender" gorm:"not null; type:varchar(250);"`
	PushID   string `json:"push_id" gorm:"not null; type:varchar(250);"`
	Status   int    `json:"status" gorm:"not null; type:int(11);"`
	Approver int    `json:"approver" gorm:"type:int(11);"`
	Reason   string `json:"reason" gorm:"type:varchar(250);"`
	CfsID    int    `json:"cfs_id" gorm:"type:int(11);"`
}

// TableName set Confession's table name to be `confessions`
func (Confession) TableName() string {
	return "confessions"
}

// FetchAll ...
func (c *Confession) FetchAll(numLoad int) []Confession {
	db := config.GetDatabaseConnection()

	var confessions []Confession
	db.Order("id desc").Limit(numLoad).Find(&confessions)

	return confessions
}

// FetchByID ...
func (c *Confession) FetchByID() error {
	db := config.GetDatabaseConnection()

	if err := db.Where("id = ?", c.ID).Find(&c).Error; err != nil {
		return errors.New("Could not find the confession")
	}

	return nil
}

// Create ...
func (c *Confession) Create() error {
	db := config.GetDatabaseConnection()

	// Validate record
	if !db.NewRecord(c) { // => returns `true` as primary key is blank
		return errors.New("New records can not have primary key id")
	}

	if err := db.Create(&c).Error; err != nil {
		return errors.New("Could not create confession")
	}

	return nil
}

// Save ...
func (c *Confession) Save() error {
	db := config.GetDatabaseConnection()

	if db.NewRecord(c) {
		if err := db.Create(&c).Error; err != nil {
			return errors.New("Could not create confessions")
		}
	} else {
		if err := db.Save(&c).Error; err != nil {
			return errors.New("Could not update confessions")
		}
	}
	return nil
}

// FetchBySender ...
func (c *Confession) FetchBySender(sender string, numLoad int) []Confession {
	db := config.GetDatabaseConnection()

	var confessions []Confession
	db.Order("id desc").Limit(numLoad).Where("sender = ?", sender).Find(&confessions)

	return confessions
}

// FetchOverview ...
func (c *Confession) FetchOverview() (int, int, int) {
	db := config.GetDatabaseConnection()

	totalCount, pendingCount, rejectedCount := 0, 0, 0
	db.Model(&Confession{}).Count(&totalCount)
	db.Model(&Confession{}).Where("status = ?", 0).Count(&pendingCount)
	db.Model(&Confession{}).Where("status = ?", 2).Count(&rejectedCount)

	return totalCount, pendingCount, rejectedCount
}

// FetchApprovedConfession ...
func (c *Confession) FetchApprovedConfession(numLoad int) []Confession {
	db := config.GetDatabaseConnection()

	var confessions []Confession
	db.Order("id desc").Limit(numLoad).Where("status = 1").Find(&confessions)

	return confessions
}

// GetNextConfessionID ...
func (c *Confession) GetNextConfessionID() int {
	db := config.GetDatabaseConnection()
	db.Order("cfs_id desc").Take(&c)
	return c.CfsID + 1
}

func (c *Confession) setConfessionApproved(status int, approver int, cfsID int) {
	c.Status = status
	c.Approver = approver
	c.CfsID = cfsID
}

// ApproveConfession ...
func (c *Confession) ApproveConfession(approverID int) error {
	if err := c.FetchByID(); err != nil {
		return errors.New("Could not find the confession")
	}

	if c.Status != 0 {
		return errors.New("Status of confession must be pending to be approved")
	}

	confessions := new(Confession)

	c.setConfessionApproved(1, approverID, confessions.GetNextConfessionID())

	if err := c.Save(); err != nil {
		return errors.New("Unable to update approved confession`")
	}

	// Send push
	pushID := c.PushID
	jsonStr := `{"notification":{"title":"Confess đã được duyệt","body":"Thật tuyệt vời!","click_action":"http://fptu.tech/my-confess","icon":"https://fptu.tech/assets/images/fptuhcm-confessions.png"},"to":"` + pushID + `"}`
	client := &http.Client{}
	req, _ := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", strings.NewReader(jsonStr))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "key=AAAARBubfwc:APA91bF18JVA5FjdP7CBOB34nVs21W7AMRZJdU3JGkYvogweo2h8BqYJ-hno2HeVsCIKu__kKaqkOYakpRydPBm4JuOnF0xFuzUENZzMLZ13JMVaaM7Zd55wRr8C4i_IErWagz8FiGaY")
	client.Do(req)

	return nil
}

func (c *Confession) setConfessionUnapproved() {
	c.Status = 0
	c.Approver = 0
	c.CfsID = 0
}

// RollbackApproveConfession ...
func (c *Confession) RollbackApproveConfession(approverID int) error {
	if err := c.FetchByID(); err != nil {
		return errors.New("Could not find the confession")
	}

	c.setConfessionUnapproved()

	if err := c.Save(); err != nil {
		return errors.New("Unable to rollback approved confession`")
	}

	return nil
}

func (c *Confession) setConfessionRejected(status int, approver int, reason string) {
	c.Status = status
	c.Approver = approver
	c.Reason = reason
}

func (c *Confession) setPushID(pushID string) {
	c.PushID = pushID
}

// RejectConfession ...
func (c *Confession) RejectConfession(approverID int, reason string) error {
	if err := c.FetchByID(); err != nil {
		return errors.New("Could not find the confession")
	}

	if c.Status != 0 {
		return errors.New("Status of confession must be pending to be rejected")
	}

	c.setConfessionRejected(2, approverID, reason)

	if err := c.Save(); err != nil {
		return errors.New("Unable to update approved confession`")
	}

	// Send push
	pushID := c.PushID
	jsonStr := `{"notification":{"title":"Confess đã được duyệt","body":"Thật tuyệt vời!","click_action":"http://fptu.tech/my-confess","icon":"https://fptu.tech/assets/images/fptuhcm-confessions.png"},"to":"` + pushID + `"}`
	client := &http.Client{}
	req, _ := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", strings.NewReader(jsonStr))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "key=AAAARBubfwc:APA91bF18JVA5FjdP7CBOB34nVs21W7AMRZJdU3JGkYvogweo2h8BqYJ-hno2HeVsCIKu__kKaqkOYakpRydPBm4JuOnF0xFuzUENZzMLZ13JMVaaM7Zd55wRr8C4i_IErWagz8FiGaY")
	client.Do(req)

	return nil
}

// SearchConfession ...
func (c *Confession) SearchConfession(keyword string) []Confession {
	db := config.GetDatabaseConnection()

	var confessions []Confession
	db.Order("id desc").Limit(50).Where("status = 1 AND content LIKE?", "%"+keyword+"%").Find(&confessions)

	return confessions
}

// SyncPushID ...
func (c *Confession) SyncPushID(sender string, pushID string) {
	db := config.GetDatabaseConnection()

	db.Model(&c).Where("sender = ?", sender).Update("push_id", pushID)
}
