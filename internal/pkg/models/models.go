package models

import (
	"encoding/json"
	"time"
)

//User - struct for handling users
type User struct {
	Name     string `json:"login"`    //Unique user name
	Password string `json:"password"` //Hashed password
	Random   string `json:"-"`        //Random IV
}

//Balance - struct for handling balance
type Balance struct {
	Balance    float32 `json:"current"`   //Accrual balace
	Withdrawns float32 `json:"withdrawn"` //Sum of withdrawns
}

type Accrual struct {
	Order  string  `json:"order"`             //Order number.
	Status string  `json:"status"`            //Order status. Allowed values are "REGISTERED", "INVALID", "PROCESSING", "PROCESSED". Status "INVALID" or "PROCESSED" are final.
	Value  float32 `json:"accrual,omitempty"` //Calculated accrual value.
}

//Order - struct for handling orders
type Order struct {
	Number  string    `json:"number"`      //Unique order number
	Status  string    `json:"status"`      //Order status. Availible states: NEW, PROCESSING, INVALID, PROCESSED
	AccRual float32   `json:"accrual"`     //Calculated bonus value
	Upload  time.Time `json:"uploaded_at"` //Order time. Time in format RFC3339
}

//MarshalJSON - marshaling time.Time to time string in RFC3339 format
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

//Withdrawn - struct for handling withdrawns
type Withdrawn struct {
	Name      string    `json:"user"`         //Username
	Number    string    `json:"number"`       //Order number
	Processed time.Time `json:"processed_at"` //Order processing time. Time in format RFC3339
	Withdrawn float32   `json:"sum"`          //Witdrawn sum
}

//MarshalJSON - marshaling time.Time to time string in RFC3339 format
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
