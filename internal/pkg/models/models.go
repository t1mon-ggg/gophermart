package models

import (
	"encoding/json"
	"time"
)

type User struct {
	Name     string `json:"login"`    //Unique user name
	Password string `json:"password"` //Hashed password
	Random   string `json:"-"`        //Random IV
}

type Order struct {
	Number  int       `json:"number"`            //Unique order number
	Status  string    `json:"status"`            //Order status. Availible states: NEW, PROCESSING, INVALID, PROCESSED
	AccRual int       `json:"accrual,omitempty"` //Calculated bonus value
	Upload  time.Time `json:"uploaded_at"`       //Order time. Time in format RFC3339
}

func (o *Order) MarshalJSON() ([]byte, error) {
	type Alias Order
	return json.Marshal(&struct {
		*Alias
		Upload string `json:"uploaded_at"`
	}{
		Alias:  (*Alias)(o),
		Upload: o.Upload.Format(time.RFC3339),
	})
}
