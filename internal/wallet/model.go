package wallet

import "time"

type Wallet struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    Balance   int64     `json:"balance"`
    Locked    int64     `json:"locked_amount"`
    Escrow    int64     `json:"escrow"`
    CreatedAt time.Time `json:"created_at"`
}
// Withdrawal model
type Withdrawal struct {
    ID        string  `json:"id"`
    UserID    string  `json:"user_id"`
    Amount    float64 `json:"amount"`
    Status    string  `json:"status"`
    CreatedAt string  `json:"created_at"`
}
