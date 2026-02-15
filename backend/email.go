package main

import (
	"fmt"
	"log"
	"net/mail"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/tools/mailer"
)

// configurePocketBaseSMTP configures PocketBase's SMTP settings for SendGrid
func configurePocketBaseSMTP(app *pocketbase.PocketBase) {
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	if smtpPassword == "" {
		log.Println("[SMTP] No SMTP_PASSWORD configured, skipping SMTP setup")
		return
	}

	settings := app.Settings()

	// Check if already configured correctly
	if settings.SMTP.Enabled && settings.SMTP.Host == "smtp.sendgrid.net" && !settings.SMTP.TLS && settings.Meta.SenderAddress == "hello@wearetheoutlook.com.au" {
		log.Println("[SMTP] Already configured correctly")
		return
	}

	settings.SMTP.Enabled = true
	settings.SMTP.Host = "smtp.sendgrid.net"
	settings.SMTP.Port = 587
	settings.SMTP.Username = "apikey"
	settings.SMTP.Password = smtpPassword
	settings.SMTP.TLS = false

	// Sender info
	settings.Meta.SenderName = "The Outlook"
	settings.Meta.SenderAddress = "hello@wearetheoutlook.com.au"

	if err := app.Save(settings); err != nil {
		log.Printf("[SMTP] Failed to save settings: %v", err)
	} else {
		log.Println("[SMTP] Settings saved successfully")
	}
}

// wrapEmailHTML wraps content in the branded email template matching the website design.
func wrapEmailHTML(content string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif; line-height: 1.3; color: #202020; font-size: 16px; margin: 0; padding: 0; background: #ffffff;">

    <div style="text-align: center; background: #ffffff; max-width: 660px; margin: auto; padding: 24px;">
        <a href="https://theoutlook.io">
            <img src="https://wearetheoutlook.com.au/themes/custom/do_website/templates/email/the-outlook-logo.png" alt="The Outlook" width="180" style="margin: 20px auto;">
        </a>
    </div>

    <div style="max-width: 660px; margin: auto; padding: 24px; background: #f8f8f8;">
        <div style="background: #ffffff; padding: 24px; border-radius: 8px;">
` + content + `
        </div>
    </div>

    <div style="max-width: 660px; margin: auto; background: #0d0d0d; padding: 40px 20px 0 20px; text-align: center; color: #ffffff;">
        <div>
            <a href="https://theoutlook.io/about" style="color: #ffffff; font-size: 14px; margin: 0 10px; text-decoration: none;">ABOUT</a>
            <a href="https://theoutlook.io/about/partnerships" style="color: #ffffff; font-size: 14px; margin: 0 10px; text-decoration: none;">PARTNERSHIPS</a>
            <a href="https://theoutlook.io/contact-us" style="color: #ffffff; font-size: 14px; margin: 0 10px; text-decoration: none;">CONTACT</a>
        </div>
        <div style="margin-top: 40px;">
            <img src="https://wearetheoutlook.com.au/themes/custom/do_website/templates/email/aboriginal-flag.png" alt="Aboriginal Flag" style="max-width: 40px; height: auto; margin: 10px;">
            <img src="https://wearetheoutlook.com.au/themes/custom/do_website/templates/email/torres-strait-islander-flag.png" alt="Torres Strait Islander Flag" style="max-width: 40px; height: auto; margin: 10px;">
            <p style="font-size: 14px; color: #777; margin-top: 10px; line-height: 1.5;">
                The Outlook acknowledges Aboriginal Traditional Owners of Country throughout Australia and pays respect to their cultures and Elders past and present.
            </p>
        </div>
        <img src="https://wearetheoutlook.com.au/themes/custom/do_website/templates/email/horizon-o.png" alt="The Outlook" style="margin: 50px auto 0 auto; width: 200px;">
    </div>

    <div style="max-width: 660px; margin: auto; padding: 10px; border-top: 1px solid #777; text-align: center; background: #0d0d0d;">
        <p style="margin-top: 10px; font-size: 12px; color: #777;">Copyright &copy; 2021-2026 Design Outlook PTY LTD &mdash; ABN 72 655 333 403</p>
    </div>

</body>
</html>`
}

// sendOTPEmail sends a 6-digit OTP code to the share recipient.
func sendOTPEmail(app *pocketbase.PocketBase, email, recipientName, code string) error {
	name := recipientName
	if name == "" {
		name = "there"
	}

	subject := "Your verification code"
	content := fmt.Sprintf(`
            <p style="color: #4a4a4a; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hi %s,</p>
            <p style="color: #4a4a4a; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">
                Your verification code to view the guest list is:
            </p>
            <div style="font-size: 32px; letter-spacing: 8px; text-align: center; padding: 24px; background: #f5f5f5; border-radius: 8px; margin: 24px 0; color: #202020;">
                %s
            </div>
            <p style="color: #9a9a9a; font-size: 14px; margin: 24px 0 0 0;">This code expires in 10 minutes.</p>
`, name, code)

	msg := &mailer.Message{
		From:    mail.Address{Address: app.Settings().Meta.SenderAddress, Name: app.Settings().Meta.SenderName},
		To:      []mail.Address{{Address: email, Name: recipientName}},
		Subject: subject,
		HTML:    wrapEmailHTML(content),
	}

	if err := app.NewMailClient().Send(msg); err != nil {
		log.Printf("[Email] Failed to send OTP to %s: %v", email, err)
		return err
	}

	log.Printf("[Email] OTP sent to %s", email)
	return nil
}

// sendShareNotificationEmail notifies the recipient that a guest list has been shared with them.
func sendShareNotificationEmail(app *pocketbase.PocketBase, recipientEmail, recipientName, shareURL, listName, eventName string) error {
	name := recipientName
	if name == "" {
		name = "there"
	}

	eventLine := ""
	if eventName != "" {
		eventLine = fmt.Sprintf(`
            <p style="color: #4a4a4a; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Event: %s</p>`, eventName)
	}

	firstName := strings.Fields(name)[0]
	subject := fmt.Sprintf("%s, you've been invited to review %s", firstName, listName)
	content := fmt.Sprintf(`
            <p style="color: #4a4a4a; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hi %s,</p>
            <p style="color: #4a4a4a; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">
                You've been invited to review the guest list <strong>%s</strong>.
            </p>
            %s
            <p style="color: #4a4a4a; font-size: 16px; line-height: 1.6; margin: 0 0 24px 0;">
                You'll need to verify your email to access the list. The link expires in 30 days.
            </p>
            <div style="text-align: center; margin: 32px 0;">
                <a href="%s" style="display: inline-block; background: #0d0d0d; color: #ffffff; padding: 14px 32px; text-decoration: none; border-radius: 6px; font-size: 16px;">
                    View guest list
                </a>
            </div>
            <p style="color: #9a9a9a; font-size: 14px; margin: 24px 0 8px 0;">
                Copy and paste if the link doesn't work:
            </p>
            <div style="background: #f5f5f5; padding: 12px 16px; border-radius: 6px; margin: 0;">
                <p style="color: #666666; font-size: 13px; font-family: 'Courier New', Courier, monospace; word-break: break-all; margin: 0;">
                    %s
                </p>
            </div>
`, name, listName, eventLine, shareURL, shareURL)

	msg := &mailer.Message{
		From:    mail.Address{Address: app.Settings().Meta.SenderAddress, Name: app.Settings().Meta.SenderName},
		To:      []mail.Address{{Address: recipientEmail, Name: recipientName}},
		Subject: subject,
		HTML:    wrapEmailHTML(content),
	}

	if err := app.NewMailClient().Send(msg); err != nil {
		log.Printf("[Email] Failed to send share notification to %s: %v", recipientEmail, err)
		return err
	}

	log.Printf("[Email] Share notification sent to %s for list %s", recipientEmail, listName)
	return nil
}

// sendAttendeeOTPEmail sends a 6-digit OTP code for attendee login.
func sendAttendeeOTPEmail(app *pocketbase.PocketBase, email, recipientName, code string) error {
	name := recipientName
	if name == "" {
		name = "there"
	}

	subject := "Your login code"
	content := fmt.Sprintf(`
            <p style="color: #4a4a4a; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hi %s,</p>
            <p style="color: #4a4a4a; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">
                Your login code is:
            </p>
            <div style="font-size: 32px; letter-spacing: 8px; text-align: center; padding: 24px; background: #f5f5f5; border-radius: 8px; margin: 24px 0; color: #202020;">
                %s
            </div>
            <p style="color: #9a9a9a; font-size: 14px; margin: 24px 0 0 0;">This code expires in 10 minutes.</p>
`, name, code)

	msg := &mailer.Message{
		From:    mail.Address{Address: app.Settings().Meta.SenderAddress, Name: app.Settings().Meta.SenderName},
		To:      []mail.Address{{Address: email, Name: recipientName}},
		Subject: subject,
		HTML:    wrapEmailHTML(content),
	}

	if err := app.NewMailClient().Send(msg); err != nil {
		log.Printf("[Email] Failed to send attendee OTP to %s: %v", email, err)
		return err
	}

	log.Printf("[Email] Attendee OTP sent to %s", email)
	return nil
}

// sendRSVPInviteEmail sends an RSVP invitation to a guest with their personal RSVP link.
func sendRSVPInviteEmail(app *pocketbase.PocketBase, recipientEmail, recipientName, rsvpURL, listName, eventName string) error {
	name := recipientName
	if name == "" {
		name = "there"
	}

	eventContext := listName
	if eventName != "" {
		eventContext = eventName
	}

	subject := fmt.Sprintf("%s, you're invited to %s", name, eventContext)
	content := fmt.Sprintf(`
            <p style="color: #4a4a4a; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hi %s,</p>
            <p style="color: #4a4a4a; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">
                You're invited to <strong>%s</strong>. Please let us know if you can make it.
            </p>
            <div style="text-align: center; margin: 32px 0;">
                <a href="%s" style="display: inline-block; background: #0d0d0d; color: #ffffff; padding: 14px 32px; text-decoration: none; border-radius: 6px; font-size: 16px;">
                    Respond now
                </a>
            </div>
            <p style="color: #9a9a9a; font-size: 14px; margin: 24px 0 8px 0;">
                Copy and paste if the link doesn't work:
            </p>
            <div style="background: #f5f5f5; padding: 12px 16px; border-radius: 6px; margin: 0;">
                <p style="color: #666666; font-size: 13px; font-family: 'Courier New', Courier, monospace; word-break: break-all; margin: 0;">
                    %s
                </p>
            </div>
            <p style="color: #9a9a9a; font-size: 14px; margin: 24px 0 0 0;">
                This link is personal to you. Please do not share it.
            </p>
`, name, eventContext, rsvpURL, rsvpURL)

	msg := &mailer.Message{
		From:    mail.Address{Address: app.Settings().Meta.SenderAddress, Name: app.Settings().Meta.SenderName},
		To:      []mail.Address{{Address: recipientEmail, Name: recipientName}},
		Subject: subject,
		HTML:    wrapEmailHTML(content),
	}

	if err := app.NewMailClient().Send(msg); err != nil {
		log.Printf("[Email] Failed to send RSVP invite to %s: %v", recipientEmail, err)
		return err
	}

	log.Printf("[Email] RSVP invite sent to %s for %s", recipientEmail, eventContext)
	return nil
}
