package bojet

// Messages holds all user-facing strings. Pass a partial Messages to
// WithMessages — only non-empty fields override the defaults.
type Messages struct {
	Welcome             string
	SharePhoneButton    string
	ContactAdminButton  string
	NotAuthorized       string
	RegistrationPending string
	Approved            string
	Rejected            string
	ContactAdminPrompt  string
	MessageSent         string
	MessageSendFailed   string
	ReplyDelivered      string
	ReplyFailed         string
	UnknownCommand      string
	GenericError        string
}

// DefaultMessages is the out-of-the-box English message set.
var DefaultMessages = Messages{
	Welcome:             "Welcome! Please share your phone number:",
	SharePhoneButton:    "📱 Share phone number",
	ContactAdminButton:  "📞 Contact Admin",
	NotAuthorized:       "⛔ You are not authorized. Please wait for admin approval.",
	RegistrationPending: "✅ Your request has been submitted. Please wait for admin approval.",
	Approved:            "🎉 Your request has been approved! You can now use the bot.",
	Rejected:            "🚫 Sorry, your request was rejected.",
	ContactAdminPrompt:  "✍️ Please type or record your message for the admin.",
	MessageSent:         "✅ Your message has been sent to the admin.",
	MessageSendFailed:   "⚠️ Failed to forward message to admin.",
	ReplyDelivered:      "✅ Reply delivered.",
	ReplyFailed:         "⚠️ Failed to send reply to user.",
	UnknownCommand:      "⚠️ Unknown command",
	GenericError:        "⚠️ An error has occurred, please try again later.",
}
