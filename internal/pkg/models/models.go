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

type Balance struct {
	Balance    float32 `json:"current"`   //Accrual balace
	Withdrawns float32 `json:"withdrawn"` //Sum of withdrawns
}

type Order struct {
	Number  string    `json:"number"`      //Unique order number
	Status  string    `json:"status"`      //Order status. Availible states: NEW, PROCESSING, INVALID, PROCESSED
	AccRual float32   `json:"accrual"`     //Calculated bonus value
	Upload  time.Time `json:"uploaded_at"` //Order time. Time in format RFC3339
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

type Withdrawn struct {
	Name      string    `json:"user"` //Unique order number
	Number    string    `json:"number"`
	Processed time.Time `json:"processed_at"` //Order processing time. Time in format RFC3339
	Withdrawn float32   `json:"sum"`
}

func (w *Withdrawn) MarshalJSON() ([]byte, error) {
	type Alias Withdrawn
	return json.Marshal(&struct {
		*Alias
		Processed string `json:"processed_at"`
	}{
		Alias:     (*Alias)(w),
		Processed: w.Processed.Format(time.RFC3339),
	})
}
