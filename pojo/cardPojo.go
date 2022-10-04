package pojo

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Idcard struct {
	Id           primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IdNo         int64              `bson:"id_no,omitempty" json:"id_no,omitempty"`
	Name         string             `bson:"name,omitempty" json:"name,omitempty"`
	Age          int64              `bson:"age,omitempty" json:"age,omitempty"`
	DOB          time.Time          `bson:"dob,omitempty" json:"dob,omitempty"`
	BloodGroup   string             `bson:"blood_group,omitempty" json:"blood_group,omitempty"`
	Designation  string             `bson:"designation,omitempty" json:"designation,omitempty"`
	JoiningDate  time.Time          `bson:"joining_date,omitempty" json:"joining_date,omitempty"`
	CreatedDate  time.Time          `bson:"created_date,omitempty" json:"created_date,omitempty"`
	FileLocation []string           `bson:"file_location,omitempty" json:"file_location,omitempty"`
	Active       bool               `bson:"active,omitempty" json:"active,omitempty"`
}

type Request struct {
	Name        string `bson:"name,omitempty" json:"name,omitempty"`
	Age         int64  `bson:"age,omitempty" json:"age,omitempty"`
	DOB         string `bson:"dob,omitempty" json:"dob,omitempty"`
	BloodGroup  string `bson:"blood_group,omitempty" json:"blood_group,omitempty"`
	Designation string `bson:"designation,omitempty" json:"designation,omitempty"`
	JoiningDate string `bson:"joining_date,omitempty" json:"joining_date,omitempty"`
}
