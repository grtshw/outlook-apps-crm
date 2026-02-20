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

// wrapRSVPEmailHTML wraps content in a dark, moody email template matching the RSVP page design.
func wrapRSVPEmailHTML(content string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif; line-height: 1.3; color: #ffffff; font-size: 16px; margin: 0; padding: 0; background: #020202;">

    <div style="max-width: 600px; margin: 0 auto; background: #020202;">
        <!-- Logos -->
        <div style="padding: 32px 32px 24px 32px;">
            <table role="presentation" cellspacing="0" cellpadding="0" border="0"><tr>
                <td style="padding-right: 16px; vertical-align: middle;"><img src="https://crm.theoutlook.io/images/logo-white.svg" alt="The Outlook" style="height: 20px; display: block;"></td>
                <td style="vertical-align: middle;"><img src="https://crm.theoutlook.io/images/to-after-dark-white.png" alt="The Outlook After Dark" style="height: 32px; display: block;"></td>
            </tr></table>
        </div>

        <!-- Hero image -->
        <img src="https://crm.theoutlook.io/images/rsvp-hero.jpg" alt="" width="600" style="width: 100%; height: auto; display: block;">

        <!-- Content -->
        <div style="padding: 40px 32px;">
` + content + `
        </div>

        <!-- Footer -->
        <div style="padding: 32px; border-top: 1px solid #333;">
            <p style="font-size: 12px; color: #777; margin: 0 0 16px 0; line-height: 1.5;">
                The Outlook acknowledges Aboriginal Traditional Owners of Country throughout Australia and pays respect to their cultures and Elders past and present.
            </p>
            <p style="font-size: 11px; color: #555; margin: 0;">
                &copy; 2021-2026 The Outlook Pty Ltd &mdash; ABN 72 655 333 403
            </p>
        </div>
    </div>

</body>
</html>`
}

// sendRSVPInviteEmail sends an RSVP invitation to a guest with their personal RSVP link.
func sendRSVPInviteEmail(app *pocketbase.PocketBase, recipientEmail, recipientName, rsvpURL, listName, listDescription, eventName, eventDate, eventTime, eventLocation string) error {
	name := recipientName
	if name == "" {
		name = "there"
	}

	firstName := strings.Fields(name)[0]

	eventContext := listName
	if eventName != "" {
		eventContext = eventName
	}

	// Build event details block
	detailsHTML := ""
	if eventDate != "" || eventTime != "" || eventLocation != "" {
		detailsHTML = `<div style="padding: 24px 0; margin: 24px 0; border-top: 1px solid #333; border-bottom: 1px solid #333;">`
		if eventDate != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0 0 4px 0;">%s</p>`, eventDate)
		}
		if eventTime != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0 0 4px 0;">%s</p>`, eventTime)
		}
		if eventLocation != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0;">%s</p>`, eventLocation)
		}
		detailsHTML += `</div>`
	}

	// Use guest list description if available, otherwise fall back to generic copy
	descriptionHTML := ""
	if listDescription != "" {
		descriptionHTML = fmt.Sprintf(`<p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">%s</p>
            <p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 8px 0;">We'd love to see you there.</p>`, listDescription)
	} else {
		descriptionHTML = `<p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 8px 0;">We'd love for you to join us for an evening of conversation, connection and great food.</p>`
	}

	subject := fmt.Sprintf("%s, you're invited to %s", firstName, eventContext)
	content := fmt.Sprintf(`
            <p style="color: rgba(255,255,255,0.5); font-size: 12px; text-transform: uppercase; letter-spacing: 2px; margin: 0 0 16px 0;">You're invited</p>
            <h1 style="color: #ffffff; font-size: 32px; line-height: 1.1; margin: 0 0 20px 0;">%s</h1>
            <p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hi %s,</p>
            %s
            %s
            <p style="color: rgba(255,255,255,0.6); font-size: 15px; line-height: 1.6; margin: 0 0 32px 0;">
                Spaces are limited, so please let us know if you can make it.
            </p>
            <!--[if mso]>
            <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%"><tr>
            <td width="50%%" style="padding-right: 6px;">
            <![endif]-->
            <div style="display: inline-block; width: 48%%; vertical-align: top;">
                <a href="%s" style="display: block; background: #E95139; color: #ffffff; padding: 16px 12px; text-decoration: none; font-size: 14px; text-align: center; border: 1px solid #E95139;">
                    I can make it
                </a>
            </div>
            <!--[if mso]>
            </td><td width="50%%" style="padding-left: 6px;">
            <![endif]-->
            <div style="display: inline-block; width: 48%%; vertical-align: top;">
                <a href="%s" style="display: block; background: transparent; color: #ffffff; padding: 16px 12px; text-decoration: none; font-size: 14px; text-align: center; border: 1px solid #555;">
                    I can't make it
                </a>
            </div>
            <!--[if mso]>
            </td></tr></table>
            <![endif]-->
            <p style="color: rgba(255,255,255,0.8); font-size: 13px; margin: 16px 0 24px 0;">
                This link is personal to you. Please don't share it.
            </p>
            <p style="color: rgba(255,255,255,0.3); font-size: 13px; margin: 0 0 8px 0;">
                Copy and paste if the link doesn't work:
            </p>
            <div style="background: rgba(255,255,255,0.05); padding: 12px 16px; margin: 0;">
                <p style="color: rgba(255,255,255,0.4); font-size: 12px; font-family: 'Courier New', Courier, monospace; word-break: break-all; margin: 0;">
                    %s
                </p>
            </div>
`, eventContext, firstName, descriptionHTML, detailsHTML, rsvpURL, rsvpURL, rsvpURL)

	msg := &mailer.Message{
		From:    mail.Address{Address: app.Settings().Meta.SenderAddress, Name: app.Settings().Meta.SenderName},
		To:      []mail.Address{{Address: recipientEmail, Name: recipientName}},
		Bcc:     []mail.Address{{Address: "hello@wearetheoutlook.com.au"}},
		Subject: subject,
		HTML:    wrapRSVPEmailHTML(content),
	}

	if err := app.NewMailClient().Send(msg); err != nil {
		log.Printf("[Email] Failed to send RSVP invite to %s: %v", recipientEmail, err)
		return err
	}

	log.Printf("[Email] RSVP invite sent to %s for %s", recipientEmail, eventContext)
	return nil
}

// sendRSVPForwardEmail sends an invitation email when someone forwards their RSVP to another person.
// To: recipient, CC: forwarder, BCC: hello@wearetheoutlook.com.au
func sendRSVPForwardEmail(app *pocketbase.PocketBase, recipientEmail, recipientName, forwarderName, forwarderEmail, rsvpURL, listDescription, eventName, eventDate, eventTime, eventLocation string) error {
	name := recipientName
	if name == "" {
		name = "there"
	}
	firstName := strings.Fields(name)[0]
	forwarderFirst := strings.Fields(forwarderName)[0]

	// Build event details block
	detailsHTML := ""
	if eventDate != "" || eventTime != "" || eventLocation != "" {
		detailsHTML = `<div style="padding: 24px 0; margin: 24px 0; border-top: 1px solid #333; border-bottom: 1px solid #333;">`
		if eventDate != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0 0 4px 0;">%s</p>`, eventDate)
		}
		if eventTime != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0 0 4px 0;">%s</p>`, eventTime)
		}
		if eventLocation != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0;">%s</p>`, eventLocation)
		}
		detailsHTML += `</div>`
	}

	// Description block after the forwarding intro
	descriptionHTML := ""
	if listDescription != "" {
		descriptionHTML = fmt.Sprintf(`<p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 16px 0 8px 0;">%s We'd love to see you there.</p>`, listDescription)
	}

	subject := fmt.Sprintf("%s thinks you'd love %s", forwarderFirst, eventName)
	content := fmt.Sprintf(`
            <p style="color: rgba(255,255,255,0.5); font-size: 12px; text-transform: uppercase; letter-spacing: 2px; margin: 0 0 16px 0;">You're invited</p>
            <h1 style="color: #ffffff; font-size: 32px; line-height: 1.1; margin: 0 0 20px 0;">%s</h1>
            <p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hi %s,</p>
            <p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 8px 0;">
                %s thought you'd enjoy %s, and has passed along an invitation for you.
            </p>
            %s
            %s
            <p style="color: rgba(255,255,255,0.6); font-size: 15px; line-height: 1.6; margin: 0 0 32px 0;">
                Spaces are limited, so let us know if you can make it.
            </p>
            <!--[if mso]>
            <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%"><tr>
            <td width="50%%" style="padding-right: 6px;">
            <![endif]-->
            <div style="display: inline-block; width: 48%%; vertical-align: top;">
                <a href="%s" style="display: block; background: #E95139; color: #ffffff; padding: 16px 12px; text-decoration: none; font-size: 14px; text-align: center; border: 1px solid #E95139;">
                    I can make it
                </a>
            </div>
            <!--[if mso]>
            </td><td width="50%%" style="padding-left: 6px;">
            <![endif]-->
            <div style="display: inline-block; width: 48%%; vertical-align: top;">
                <a href="%s" style="display: block; background: transparent; color: #ffffff; padding: 16px 12px; text-decoration: none; font-size: 14px; text-align: center; border: 1px solid #555;">
                    I can't make it
                </a>
            </div>
            <!--[if mso]>
            </td></tr></table>
            <![endif]-->
            <p style="color: rgba(255,255,255,0.3); font-size: 13px; margin: 24px 0 8px 0;">
                Copy and paste if the link doesn't work:
            </p>
            <div style="background: rgba(255,255,255,0.05); padding: 12px 16px; margin: 0;">
                <p style="color: rgba(255,255,255,0.4); font-size: 12px; font-family: 'Courier New', Courier, monospace; word-break: break-all; margin: 0;">
                    %s
                </p>
            </div>
`, eventName, firstName, forwarderFirst, eventName, descriptionHTML, detailsHTML, rsvpURL, rsvpURL, rsvpURL)

	msg := &mailer.Message{
		From:    mail.Address{Address: app.Settings().Meta.SenderAddress, Name: app.Settings().Meta.SenderName},
		To:      []mail.Address{{Address: recipientEmail, Name: recipientName}},
		Cc:      []mail.Address{{Address: forwarderEmail, Name: forwarderName}},
		Bcc:     []mail.Address{{Address: "hello@wearetheoutlook.com.au"}},
		Subject: subject,
		HTML:    wrapRSVPEmailHTML(content),
	}

	if err := app.NewMailClient().Send(msg); err != nil {
		log.Printf("[Email] Failed to send forward invite to %s: %v", recipientEmail, err)
		return err
	}

	log.Printf("[Email] Forward invite sent to %s (forwarded by %s) for %s", recipientEmail, forwarderEmail, eventName)
	return nil
}

// sendRSVPConfirmationEmail sends a confirmation email when someone accepts an RSVP.
// Always BCCs hello@wearetheoutlook.com.au plus any additional BCC emails from the guest list config.
func sendRSVPConfirmationEmail(app *pocketbase.PocketBase, recipientEmail, recipientName, eventName, eventDate, eventTime, eventLocation string, bccEmails []string) error {
	name := recipientName
	if name == "" {
		name = "there"
	}

	firstName := strings.Fields(name)[0]

	subject := fmt.Sprintf("You're confirmed for %s", eventName)

	// Build event details block
	detailsHTML := ""
	if eventDate != "" || eventTime != "" || eventLocation != "" {
		detailsHTML = `<div style="padding: 24px 0; margin: 24px 0; border-top: 1px solid #333; border-bottom: 1px solid #333;">`
		if eventDate != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0 0 4px 0;">%s</p>`, eventDate)
		}
		if eventTime != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0 0 4px 0;">%s</p>`, eventTime)
		}
		if eventLocation != "" {
			detailsHTML += fmt.Sprintf(`<p style="color: rgba(255,255,255,0.7); font-size: 15px; margin: 0;">%s</p>`, eventLocation)
		}
		detailsHTML += `</div>`
	}

	content := fmt.Sprintf(`
            <p style="color: rgba(255,255,255,0.5); font-size: 12px; text-transform: uppercase; letter-spacing: 2px; margin: 0 0 16px 0;">You're confirmed</p>
            <h1 style="color: #ffffff; font-size: 32px; line-height: 1.1; margin: 0 0 20px 0;">%s</h1>
            <p style="color: rgba(255,255,255,0.8); font-size: 16px; line-height: 1.6; margin: 0 0 8px 0;">
                %s, confirming your RSVP and looking forward to seeing you on the night.
            </p>
            %s
            <p style="color: rgba(255,255,255,0.4); font-size: 14px; margin: 0;">
                If your plans change, please let us know by replying to this email.
            </p>
`, eventName, firstName, detailsHTML)

	// Build BCC list: always include hello@ + any configured contacts
	bccList := []mail.Address{{Address: "hello@wearetheoutlook.com.au"}}
	for _, addr := range bccEmails {
		if addr != "" {
			bccList = append(bccList, mail.Address{Address: addr})
		}
	}

	msg := &mailer.Message{
		From:    mail.Address{Address: app.Settings().Meta.SenderAddress, Name: app.Settings().Meta.SenderName},
		To:      []mail.Address{{Address: recipientEmail, Name: recipientName}},
		Bcc:     bccList,
		Subject: subject,
		HTML:    wrapRSVPEmailHTML(content),
	}

	if err := app.NewMailClient().Send(msg); err != nil {
		log.Printf("[Email] Failed to send RSVP confirmation to %s: %v", recipientEmail, err)
		return err
	}

	log.Printf("[Email] RSVP confirmation sent to %s for %s", recipientEmail, eventName)
	return nil
}

// sendPlusOneNotificationEmail sends an internal notification when someone requests a plus-one.
// Recipients are hello@ + configured BCC contacts, all as direct To recipients.
func sendPlusOneNotificationEmail(app *pocketbase.PocketBase, requesterName, plusOneName, plusOneJobTitle, plusOneCompany, plusOneEmail, eventName string, toEmails []string) error {
	subject := fmt.Sprintf("Plus-one request: %s for %s", requesterName, eventName)

	// Build plus-one details
	detailsHTML := ""
	if plusOneName != "" {
		detailsHTML += fmt.Sprintf(`<p style="margin: 0 0 4px 0;"><strong>Name:</strong> %s</p>`, plusOneName)
	}
	if plusOneJobTitle != "" {
		detailsHTML += fmt.Sprintf(`<p style="margin: 0 0 4px 0;"><strong>Job title:</strong> %s</p>`, plusOneJobTitle)
	}
	if plusOneCompany != "" {
		detailsHTML += fmt.Sprintf(`<p style="margin: 0 0 4px 0;"><strong>Company:</strong> %s</p>`, plusOneCompany)
	}
	if plusOneEmail != "" {
		detailsHTML += fmt.Sprintf(`<p style="margin: 0 0 4px 0;"><strong>Email:</strong> %s</p>`, plusOneEmail)
	}

	content := fmt.Sprintf(`
            <p style="font-size: 16px; margin: 0 0 16px 0;">
                <strong>%s</strong> would like to bring a plus-one to <strong>%s</strong>.
            </p>
            <div style="background: #f8f8f8; padding: 16px; border-radius: 8px; margin: 0 0 16px 0;">
                %s
            </div>
            <p style="font-size: 14px; color: #666; margin: 0;">
                This plus-one has been added to the guest list as "Maybe" for your review.
            </p>
`, requesterName, eventName, detailsHTML)

	// Build To list: hello@ + configured contacts
	toList := []mail.Address{{Address: "hello@wearetheoutlook.com.au"}}
	for _, addr := range toEmails {
		if addr != "" && addr != "hello@wearetheoutlook.com.au" {
			toList = append(toList, mail.Address{Address: addr})
		}
	}

	msg := &mailer.Message{
		From:    mail.Address{Address: app.Settings().Meta.SenderAddress, Name: app.Settings().Meta.SenderName},
		To:      toList,
		Subject: subject,
		HTML:    wrapEmailHTML(content),
	}

	if err := app.NewMailClient().Send(msg); err != nil {
		log.Printf("[Email] Failed to send plus-one notification for %s: %v", requesterName, err)
		return err
	}

	log.Printf("[Email] Plus-one notification sent for %s (event: %s)", requesterName, eventName)
	return nil
}
