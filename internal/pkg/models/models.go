package models

import (
	"encoding/json"
	"time"
)

type User struct {
	Name     string `json:"login"`    //Unique user name
	Password string `json:"password"` //Hashed password
	Random   string `json:"-"`        //Random IV
	Balance  int    `json:"-"`        //Accrual balace
}

type Order struct {
	Number    string    `json:"number"`                 //Unique order number
	Status    string    `json:"status,omitempty"`       //Order status. Availible states: NEW, PROCESSING, INVALID, PROCESSED
	AccRual   float32   `json:"accrual,omitempty"`      //Calculated bonus value
	Upload    time.Time `json:"uploaded_at,omitempty"`  //Order time. Time in format RFC3339
	Withdrawn float32   `json:"sum,omitempty"`          //Withdrawn applied to order
	Processed time.Time `json:"processed_at,omitempty"` //Order processing time. Time in format RFC3339
}

func (o *Order) MarshalJSON() ([]byte, error) {
	type Alias Order
	return json.Marshal(&struct {
		*Alias
		Upload    string `json:"uploaded_at,omitempty"`
		Processed string `json:"processed_at,omitempty"`
	}{
		Alias:     (*Alias)(o),
		Upload:    o.Upload.Format(time.RFC3339),
		Processed: o.Processed.Format(time.RFC3339),
	})
}
