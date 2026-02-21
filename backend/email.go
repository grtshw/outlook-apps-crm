package main

import (
	"fmt"
	"log"
	"net/mail"
	"net/url"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
)

// EmailTheme holds theme colors for RSVP emails.
type EmailTheme struct {
	OuterBackground string // outer body bg (behind the card)
	Background      string // container/card bg
	Text            string // heading + body text
	TextMuted    string // secondary text
	Border       string // divider lines
	Button       string // CTA button bg
	ButtonText   string // CTA button text
	Primary      string // accent color
	IsDark       bool
	LogoURL      string // main logo
	BrandLogoURL string // secondary brand wordmark
	HeroImageURL string // hero image
}

func defaultEmailTheme() EmailTheme {
	base := getPublicBaseURL()
	return EmailTheme{
		OuterBackground: "#020202",
		Background:      "#020202",
		Text:         "#ffffff",
		TextMuted:    "#A8A9B1",
		Border:       "#333333",
		Button:       "#E95139",
		ButtonText:   "#ffffff",
		Primary:      "#E95139",
		IsDark:       true,
		LogoURL:      base + "/images/logo-white.svg",
		BrandLogoURL: base + "/images/to-after-dark-white.png",
		HeroImageURL: base + "/images/rsvp-hero.jpg",
	}
}

// buildEmailTheme constructs an EmailTheme from a guest list's theme.
func buildEmailTheme(app *pocketbase.PocketBase, guestList *core.Record) EmailTheme {
	theme := fetchThemeForGuestList(app, guestList)
	if theme == nil {
		return defaultEmailTheme()
	}

	base := getPublicBaseURL()
	isDark := getBool(theme, "is_dark")
	et := EmailTheme{
		Background: getStr(theme, "color_background", "#020202"),
		Text:       getStr(theme, "color_text", "#ffffff"),
		TextMuted:  getStr(theme, "color_text_muted", "#A8A9B1"),
		Border:     getStr(theme, "color_border", "#333333"),
		Primary:    getStr(theme, "color_primary", "#E95139"),
		IsDark:     isDark,
	}

	// For light themes: grey outer bg, white card, dark text
	if !isDark {
		et.OuterBackground = "#F0F0F0"
		et.Background = "#ffffff"
		et.Text = "#000000"
		et.TextMuted = "#666666"
	} else {
		et.OuterBackground = et.Background
	}

	// Button color: use color_button, fall back to primary
	et.Button = getStr(theme, "color_button", "")
	if et.Button == "" {
		et.Button = et.Primary
	}

	// For light themes: black button, white text
	if !isDark {
		et.Button = "#000000"
		et.ButtonText = "#ffffff"
	} else {
		et.ButtonText = "#ffffff"
	}

	// Logos — use email-safe PNG version if the theme logo is an SVG
	logoURL := getStr(theme, "logo_url", base+"/images/logo-white.svg")
	if strings.HasSuffix(logoURL, ".svg") {
		logoURL = base + "/images/logo-email.png"
	}
	et.LogoURL = logoURL
	et.BrandLogoURL = getStr(theme, "logo_light_url", "")

	// Hero image: guest list landing_image_url > theme hero_image_url > default
	heroURL := guestList.GetString("landing_image_url")
	if heroURL == "" {
		heroURL = getStr(theme, "hero_image_url", "")
	}
	if heroURL == "" {
		heroURL = base + "/images/rsvp-hero.jpg"
	}
	et.HeroImageURL = heroURL

	return et
}

func getStr(m map[string]any, key, fallback string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// textWithOpacity returns a CSS color with opacity for email inline styles.
func textWithOpacity(hex string, opacity string) string {
	// For simplicity in email HTML, use the hex color directly at full opacity
	// or use rgba. Convert hex to rgba.
	r, g, b := hexToRGB(hex)
	return fmt.Sprintf("rgba(%d,%d,%d,%s)", r, g, b, opacity)
}

func hexToRGB(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string(hex[0]) + string(hex[0]) + string(hex[1]) + string(hex[1]) + string(hex[2]) + string(hex[2])
	}
	if len(hex) != 6 {
		return 255, 255, 255
	}
	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

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

// wrapRSVPEmailHTML wraps content in the themed RSVP email template.
func wrapRSVPEmailHTML(content string, theme EmailTheme) string {
	// Build logo section
	logoHTML := fmt.Sprintf(`<td style="vertical-align: middle;"><img src="%s" alt="The Outlook" style="height: 20px; display: block; font-weight: bold;"></td>`, theme.LogoURL)
	if theme.BrandLogoURL != "" {
		logoHTML = fmt.Sprintf(`<td style="padding-right: 16px; vertical-align: middle;"><img src="%s" alt="The Outlook" style="height: 20px; display: block; font-weight: bold;"></td>
                <td style="vertical-align: middle;"><img src="%s" alt="" style="height: 32px; display: block;"></td>`, theme.LogoURL, theme.BrandLogoURL)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif; line-height: 1.3; color: %s; font-size: 16px; margin: 0; padding: 0; background: %s;">

    <div style="max-width: 600px; margin: 0 auto; background: %s;">
        <!-- Logos -->
        <div style="padding: 32px 32px 24px 32px;">
            <table role="presentation" cellspacing="0" cellpadding="0" border="0"><tr>
                %s
            </tr></table>
        </div>

        <!-- Hero image -->
        <img src="%s" alt="" width="600" style="width: 100%%; height: auto; display: block;">

        <!-- Content -->
        <div style="padding: 40px 32px;">
%s
        </div>

        <!-- Footer -->
        <div style="padding: 32px; border-top: 1px solid %s;">
            <p style="font-size: 12px; color: #777; margin: 0 0 16px 0; line-height: 1.5;">
                The Outlook acknowledges Aboriginal Traditional Owners of Country throughout Australia and pays respect to their cultures and Elders past and present.
            </p>
            <p style="font-size: 11px; color: #555; margin: 0;">
                &copy; 2021-2026 The Outlook Pty Ltd &mdash; ABN 72 655 333 403
            </p>
        </div>
    </div>

</body>
</html>`, theme.Text, theme.OuterBackground, theme.Background, logoHTML, theme.HeroImageURL, content, theme.Border)
}

// buildEventDetailsHTML builds the event date/time/location block for RSVP emails.
func buildEventDetailsHTML(theme EmailTheme, eventDate, eventTime, eventLocation string) string {
	if eventDate == "" && eventTime == "" && eventLocation == "" {
		return ""
	}
	detailsColor := textWithOpacity(theme.Text, "0.7")
	html := fmt.Sprintf(`<div style="padding: 24px 0; margin: 24px 0; border-top: 1px solid %s; border-bottom: 1px solid %s;">`, theme.Border, theme.Border)
	if eventDate != "" {
		html += fmt.Sprintf(`<p style="color: %s; font-size: 15px; margin: 0 0 4px 0;">%s</p>`, detailsColor, eventDate)
	}
	if eventTime != "" {
		html += fmt.Sprintf(`<p style="color: %s; font-size: 15px; margin: 0 0 4px 0;">%s</p>`, detailsColor, eventTime)
	}
	if eventLocation != "" {
		html += fmt.Sprintf(`<p style="color: %s; font-size: 15px; margin: 0;">%s</p>`, detailsColor, eventLocation)
	}
	html += `</div>`
	return html
}

// buildRSVPInviteContent builds the inner content for an RSVP invite email.
func buildRSVPInviteContent(theme EmailTheme, eventContext, firstName, listDescription, eventDate, eventTime, eventLocation, rsvpURL string) string {
	detailsHTML := buildEventDetailsHTML(theme, eventDate, eventTime, eventLocation)

	descriptionHTML := ""
	if listDescription != "" {
		descriptionHTML = fmt.Sprintf(`<p style="color: %s; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">%s</p>
            <p style="color: %s; font-size: 16px; line-height: 1.6; margin: 0 0 8px 0;">We'd love to see you there.</p>`,
			textWithOpacity(theme.Text, "0.8"), listDescription, textWithOpacity(theme.Text, "0.8"))
	} else {
		descriptionHTML = fmt.Sprintf(`<p style="color: %s; font-size: 16px; line-height: 1.6; margin: 0 0 8px 0;">We'd love for you to join us for an evening of conversation, connection and great food.</p>`,
			textWithOpacity(theme.Text, "0.8"))
	}

	subtleText := textWithOpacity(theme.Text, "0.5")
	bodyText := textWithOpacity(theme.Text, "0.8")
	mutedText := textWithOpacity(theme.Text, "0.6")
	faintText := textWithOpacity(theme.Text, "0.3")
	faintBg := textWithOpacity(theme.Text, "0.05")

	return fmt.Sprintf(`
            <p style="color: %s; font-size: 12px; text-transform: uppercase; letter-spacing: 2px; margin: 0 0 16px 0;">You're invited</p>
            <h1 style="color: %s; font-size: 32px; line-height: 1.1; margin: 0 0 20px 0;">%s</h1>
            <p style="color: %s; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hi %s,</p>
            %s
            %s
            <p style="color: %s; font-size: 15px; line-height: 1.6; margin: 0 0 32px 0;">
                Spaces are limited, so please let us know if you can make it.
            </p>
            <!--[if mso]>
            <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%"><tr>
            <td width="50%%" style="padding-right: 6px;">
            <![endif]-->
            <div style="display: inline-block; width: 48%%; vertical-align: top;">
                <a href="%s" style="display: block; background: %s; color: %s; padding: 16px 12px; text-decoration: none; font-size: 14px; text-align: center; border: 1px solid %s;">
                    I can make it
                </a>
            </div>
            <!--[if mso]>
            </td><td width="50%%" style="padding-left: 6px;">
            <![endif]-->
            <div style="display: inline-block; width: 48%%; vertical-align: top;">
                <a href="%s" style="display: block; background: transparent; color: %s; padding: 16px 12px; text-decoration: none; font-size: 14px; text-align: center; border: 1px solid %s;">
                    I can't make it
                </a>
            </div>
            <!--[if mso]>
            </td></tr></table>
            <![endif]-->
            <p style="color: %s; font-size: 13px; margin: 16px 0 24px 0;">
                This link is personal to you. Please don't share it.
            </p>
            <p style="color: %s; font-size: 13px; margin: 0 0 8px 0;">
                Copy and paste if the link doesn't work:
            </p>
            <div style="background: %s; padding: 12px 16px; margin: 0;">
                <p style="color: %s; font-size: 12px; font-family: 'Courier New', Courier, monospace; word-break: break-all; margin: 0;">
                    %s
                </p>
            </div>
`,
		subtleText, theme.Text, eventContext,
		bodyText, firstName,
		descriptionHTML,
		detailsHTML,
		mutedText,
		rsvpURL, theme.Button, theme.ButtonText, theme.Button,
		rsvpURL, theme.Text, theme.Border,
		bodyText,
		faintText,
		faintBg,
		textWithOpacity(theme.Text, "0.4"),
		rsvpURL,
	)
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
// rsvpToken is used to embed open/click tracking in the email.
func sendRSVPInviteEmail(app *pocketbase.PocketBase, recipientEmail, recipientName, rsvpURL, rsvpToken, listName, listDescription, eventName, eventDate, eventTime, eventLocation string, theme EmailTheme) error {
	name := recipientName
	if name == "" {
		name = "there"
	}

	firstName := strings.Fields(name)[0]

	eventContext := listName
	if eventName != "" {
		eventContext = eventName
	}

	subject := fmt.Sprintf("%s, you're invited to %s", firstName, eventContext)
	content := buildRSVPInviteContent(theme, eventContext, firstName, listDescription, eventDate, eventTime, eventLocation, rsvpURL)

	// Add invite tracking (open pixel + click-wrapped links)
	if rsvpToken != "" {
		baseURL := getPublicBaseURL()
		trackBase := fmt.Sprintf("%s/api/public/t/%s", baseURL, rsvpToken)
		clickURL := fmt.Sprintf("%s/click?url=%s", trackBase, url.QueryEscape(rsvpURL))
		pixelURL := fmt.Sprintf("%s/open.gif", trackBase)

		// Replace rsvpURL hrefs with click-tracked URL
		content = strings.ReplaceAll(content, fmt.Sprintf(`href="%s"`, rsvpURL), fmt.Sprintf(`href="%s"`, clickURL))

		// Append tracking pixel
		content += fmt.Sprintf(`<img src="%s" width="1" height="1" alt="" style="display:block;width:1px;height:1px;border:0;" />`, pixelURL)
	}

	msg := &mailer.Message{
		From:    mail.Address{Address: app.Settings().Meta.SenderAddress, Name: app.Settings().Meta.SenderName},
		To:      []mail.Address{{Address: recipientEmail, Name: recipientName}},
		Bcc:     []mail.Address{{Address: "hello@wearetheoutlook.com.au"}},
		Subject: subject,
		HTML:    wrapRSVPEmailHTML(content, theme),
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
func sendRSVPForwardEmail(app *pocketbase.PocketBase, recipientEmail, recipientName, forwarderName, forwarderEmail, rsvpURL, listDescription, eventName, eventDate, eventTime, eventLocation string, theme EmailTheme) error {
	name := recipientName
	if name == "" {
		name = "there"
	}
	firstName := strings.Fields(name)[0]
	forwarderFirst := strings.Fields(forwarderName)[0]

	detailsHTML := buildEventDetailsHTML(theme, eventDate, eventTime, eventLocation)

	descriptionHTML := ""
	if listDescription != "" {
		descriptionHTML = fmt.Sprintf(`<p style="color: %s; font-size: 16px; line-height: 1.6; margin: 16px 0 8px 0;">%s We'd love to see you there.</p>`,
			textWithOpacity(theme.Text, "0.8"), listDescription)
	}

	subtleText := textWithOpacity(theme.Text, "0.5")
	bodyText := textWithOpacity(theme.Text, "0.8")
	mutedText := textWithOpacity(theme.Text, "0.6")
	faintText := textWithOpacity(theme.Text, "0.3")
	faintBg := textWithOpacity(theme.Text, "0.05")

	subject := fmt.Sprintf("%s thinks you'd love %s", forwarderFirst, eventName)
	content := fmt.Sprintf(`
            <p style="color: %s; font-size: 12px; text-transform: uppercase; letter-spacing: 2px; margin: 0 0 16px 0;">You're invited</p>
            <h1 style="color: %s; font-size: 32px; line-height: 1.1; margin: 0 0 20px 0;">%s</h1>
            <p style="color: %s; font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hi %s,</p>
            <p style="color: %s; font-size: 16px; line-height: 1.6; margin: 0 0 8px 0;">
                %s thought you'd enjoy %s, and has passed along an invitation for you.
            </p>
            %s
            %s
            <p style="color: %s; font-size: 15px; line-height: 1.6; margin: 0 0 32px 0;">
                Spaces are limited, so let us know if you can make it.
            </p>
            <!--[if mso]>
            <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%"><tr>
            <td width="50%%" style="padding-right: 6px;">
            <![endif]-->
            <div style="display: inline-block; width: 48%%; vertical-align: top;">
                <a href="%s" style="display: block; background: %s; color: %s; padding: 16px 12px; text-decoration: none; font-size: 14px; text-align: center; border: 1px solid %s;">
                    I can make it
                </a>
            </div>
            <!--[if mso]>
            </td><td width="50%%" style="padding-left: 6px;">
            <![endif]-->
            <div style="display: inline-block; width: 48%%; vertical-align: top;">
                <a href="%s" style="display: block; background: transparent; color: %s; padding: 16px 12px; text-decoration: none; font-size: 14px; text-align: center; border: 1px solid %s;">
                    I can't make it
                </a>
            </div>
            <!--[if mso]>
            </td></tr></table>
            <![endif]-->
            <p style="color: %s; font-size: 13px; margin: 24px 0 8px 0;">
                Copy and paste if the link doesn't work:
            </p>
            <div style="background: %s; padding: 12px 16px; margin: 0;">
                <p style="color: %s; font-size: 12px; font-family: 'Courier New', Courier, monospace; word-break: break-all; margin: 0;">
                    %s
                </p>
            </div>
`,
		subtleText, theme.Text, eventName,
		bodyText, firstName,
		bodyText, forwarderFirst, eventName,
		descriptionHTML,
		detailsHTML,
		mutedText,
		rsvpURL, theme.Button, theme.ButtonText, theme.Button,
		rsvpURL, theme.Text, theme.Border,
		faintText,
		faintBg,
		textWithOpacity(theme.Text, "0.4"),
		rsvpURL,
	)

	msg := &mailer.Message{
		From:    mail.Address{Address: app.Settings().Meta.SenderAddress, Name: app.Settings().Meta.SenderName},
		To:      []mail.Address{{Address: recipientEmail, Name: recipientName}},
		Cc:      []mail.Address{{Address: forwarderEmail, Name: forwarderName}},
		Bcc:     []mail.Address{{Address: "hello@wearetheoutlook.com.au"}},
		Subject: subject,
		HTML:    wrapRSVPEmailHTML(content, theme),
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
func sendRSVPConfirmationEmail(app *pocketbase.PocketBase, recipientEmail, recipientName, eventName, eventDate, eventTime, eventLocation string, bccEmails []string, theme EmailTheme) error {
	name := recipientName
	if name == "" {
		name = "there"
	}

	firstName := strings.Fields(name)[0]

	subject := fmt.Sprintf("You're confirmed for %s", eventName)

	detailsHTML := buildEventDetailsHTML(theme, eventDate, eventTime, eventLocation)

	subtleText := textWithOpacity(theme.Text, "0.5")
	bodyText := textWithOpacity(theme.Text, "0.8")

	content := fmt.Sprintf(`
            <p style="color: %s; font-size: 12px; text-transform: uppercase; letter-spacing: 2px; margin: 0 0 16px 0;">You're confirmed</p>
            <h1 style="color: %s; font-size: 32px; line-height: 1.1; margin: 0 0 20px 0;">%s</h1>
            <p style="color: %s; font-size: 16px; line-height: 1.6; margin: 0 0 8px 0;">
                %s, confirming your RSVP and looking forward to seeing you on the night.
            </p>
            %s
            <p style="color: %s; font-size: 14px; margin: 0;">
                If your plans change, please let us know by replying to this email.
            </p>
`, subtleText, theme.Text, eventName, bodyText, firstName, detailsHTML, textWithOpacity(theme.Text, "0.4"))

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
		HTML:    wrapRSVPEmailHTML(content, theme),
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
func sendPlusOneNotificationEmail(app *pocketbase.PocketBase, requesterName, plusOneName, plusOneJobTitle, plusOneCompany, plusOneEmail, eventName, guestListID string, toEmails []string) error {
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
            <p style="font-size: 14px; color: #666; margin: 0 0 16px 0;">
                This plus-one has been added to the guest list as "Maybe" for your review.
            </p>
            <p style="margin: 0;">
                <a href="https://crm.theoutlook.io/guest-lists/%s" style="color: #E95139; text-decoration: underline;">View guest list</a>
            </p>
`, requesterName, eventName, detailsHTML, guestListID)

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
