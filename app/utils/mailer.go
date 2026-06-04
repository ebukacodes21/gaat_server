package utils

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/jordan-wright/email"
)

var (
	// smtpAuthAddr   = "smtp.gmail.com"
	// smtpServerAddr = "smtp.gmail.com:465"

	smtpAuthAddr   = os.Getenv("SMTP_AUTH_ADDR")
	smtpServerAddr = os.Getenv("SMTP_SERVER_ADDR")
)

const EmailTemplate = `
<!DOCTYPE html>

<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
</head>
<body style="margin:0;padding:0;background-color:#121212;font-family:Arial,Helvetica,sans-serif;">

<table width="100%" cellpadding="0" cellspacing="0" style="background-color:#121212;padding:40px 20px;">
    <tr>
        <td align="center">
        <table width="600" cellpadding="0" cellspacing="0"
               style="background-color:#1A1816;border-radius:12px;overflow:hidden;">

            <!-- Header -->
            <tr>
                <td align="center"
                    style="padding:30px;border-bottom:1px solid #2A2622;">
                    <h1 style="margin:0;color:#E6A15C;font-size:24px;">
                        GAAT Investment Limited
                    </h1>
                </td>
            </tr>

            <!-- Title -->
            <tr>
                <td style="padding:30px 30px 10px 30px;">
                    <h2 style="margin:0;color:#FFFFFF;font-size:22px;">
                        {{.Title}}
                    </h2>
                </td>
            </tr>

            <!-- Body -->
            <tr>
                <td style="padding:10px 30px 20px 30px;
                           color:#E6E1DC;
                           font-size:15px;
                           line-height:1.7;">
                    {{.Body}}
                </td>
            </tr>

            <!-- Action -->
            {{if .Action}}
            <tr>
                <td align="center" style="padding:10px 30px 30px 30px;">
                    {{.Action}}
                </td>
            </tr>
            {{end}}

            <!-- Footer -->
            <tr>
                <td align="center"
                    style="padding:20px;
                           border-top:1px solid #2A2622;
                           background-color:#151311;">
                    <p style="margin:0;color:#8C8176;font-size:12px;">
                        GAAT Investment Limited
                    </p>

                    <p style="margin:8px 0 0 0;color:#6D655D;font-size:11px;">
                        © 2026 GAAT Investment Limited. All rights reserved.
                    </p>
                </td>
            </tr>

        </table>

    </td>
</tr>

</table>

</body>
</html>

`

type Mailer interface {
	SendMail(subject, content string, to, cc, bcc, attachFiles []string) error
}

type GMailer struct {
	name              string
	fromEmailAddress  string
	fromEmailPassword string
}

func NewGAATMailer(name, address, password string) Mailer {
	return &GMailer{name: name, fromEmailAddress: address, fromEmailPassword: password}
}

func (gs *GMailer) SendMail(subject, content string, to, cc, bcc, attachFiles []string) error {
	// Create the email object
	email := email.NewEmail()
	email.From = fmt.Sprintf("%s <%s>", gs.name, gs.fromEmailAddress)
	email.Subject = subject
	email.HTML = []byte(content)
	email.To = to
	email.Cc = cc
	email.Bcc = bcc

	// Set up authentication
	auth := smtp.PlainAuth("", gs.fromEmailAddress, gs.fromEmailPassword, smtpAuthAddr)

	// Dial the SMTP server directly using SSL on port 46*
	conn, err := tls.Dial("tcp", smtpServerAddr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %v", err)
	}
	defer conn.Close()

	// Create the SMTP client
	client, err := smtp.NewClient(conn, smtpAuthAddr)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %v", err)
	}
	defer client.Quit()

	// Authenticate with the server
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %v", err)
	}

	// Set the sender and recipients
	if err := client.Mail(gs.fromEmailAddress); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}
	for _, recipient := range email.To {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %v", recipient, err)
		}
	}

	// Write the email content to the connection
	dataWriter, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to create data writer: %v", err)
	}
	defer dataWriter.Close()

	// Construct the raw email with attachments
	rawEmail := buildRawEmailWithAttachments(email, attachFiles)

	// Write the raw email to the data writer
	if _, err := dataWriter.Write([]byte(rawEmail)); err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}

// Helper function to build the raw email content (headers + body + attachments)
func buildRawEmailWithAttachments(email *email.Email, attachFiles []string) string {
	var rawEmail strings.Builder

	// Boundaries for the multipart message
	boundary := fmt.Sprintf("----=_Part_%d_%s", time.Now().Unix(), "1234567890")

	// Add headers for multipart
	rawEmail.WriteString(fmt.Sprintf("From: %s\n", email.From))
	rawEmail.WriteString(fmt.Sprintf("To: %s\n", strings.Join(email.To, ", ")))
	if len(email.Cc) > 0 {
		rawEmail.WriteString(fmt.Sprintf("Cc: %s\n", strings.Join(email.Cc, ", ")))
	}
	if len(email.Bcc) > 0 {
		rawEmail.WriteString(fmt.Sprintf("Bcc: %s\n", strings.Join(email.Bcc, ", ")))
	}
	rawEmail.WriteString(fmt.Sprintf("Subject: %s\n", email.Subject))
	rawEmail.WriteString("MIME-Version: 1.0\n")
	rawEmail.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\n", boundary))
	rawEmail.WriteString("\n")

	// Start multipart message
	rawEmail.WriteString(fmt.Sprintf("--%s\n", boundary))

	// Email body (HTML content)
	rawEmail.WriteString("Content-Type: text/html; charset=UTF-8\n")
	rawEmail.WriteString("Content-Transfer-Encoding: 7bit\n")
	rawEmail.WriteString("\n")
	rawEmail.WriteString(string(email.HTML))
	rawEmail.WriteString("\n")

	// if any attachments
	if len(attachFiles) > 0 {
		for _, file := range attachFiles {
			// Add the boundary before each attachment part
			rawEmail.WriteString(fmt.Sprintf("--%s\n", boundary))
			rawEmail.WriteString("Content-Type: application/octet-stream; name=\"" + file + "\"\n")
			rawEmail.WriteString("Content-Transfer-Encoding: base64\n")
			rawEmail.WriteString("Content-Disposition: attachment; filename=\"" + file + "\"\n")
			rawEmail.WriteString("\n")

			// Read and encode file content to base64
			content, err := os.ReadFile(file)
			if err != nil {
				rawEmail.WriteString(fmt.Sprintf("Error reading attachment %s: %v\n", file, err))
				continue
			}
			encoded := base64.StdEncoding.EncodeToString(content)
			rawEmail.WriteString(encoded)
			rawEmail.WriteString("\n")
		}
	}

	// End of multipart message
	rawEmail.WriteString(fmt.Sprintf("--%s--\n", boundary))

	return rawEmail.String()
}
