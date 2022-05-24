package models

import "time"

type User struct {
	Name     string `json:"login"`    //Unique user name
	Password string `json:"password"` //Hashed password
	Random   string `json:"-"`        //Random IV
}

// type OrderAccrual struct {
// 	Number  int    `json:"order"`             //Unique order number
// 	Status  string `json:"status"`            //Order status. Availible states: REGISTERED, REGISTERED, PROCESSING, PROCESSED
// 	AccRual string `json:"accrual,omitempty"` //Calculated bonus value
// }

type Order struct {
	Number  int       `json:"number"`      //Unique order number
	Status  string    `json:"status"`      //Order status. Availible states: NEW, PROCESSING, INVALID, PROCESSED
	AccRual string    `json:"accrual"`     //Calculated bonus value
	Upload  time.Time `json:"uploaded_at"` //Order time. Time in format RFC3339
}
