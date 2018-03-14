package slack

// Message is a top level structure for slack messages
type Message struct {
	Channel     string       `json:"channel"`
	Username    string       `json:"username"`
	Emoji       string       `json:"emoji"`
	Attachments []Attachment `json:"attachments"`
}

// Attachment for slack
type Attachment struct {
	Fallback string   `json:"fallback"`
	Color    string   `json:"color"`
	Text     string   `json:"text"`
	MrkdwnIn []string `json:"mrkdwn_in"`
	Fields   []Field  `json:"fields"`
}

// Field is a struct to describe fields in slack message
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}
