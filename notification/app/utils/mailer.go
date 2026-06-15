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

const (
	smtpAuthAddr   = "smtp.gmail.com"
	smtpServerAddr = "smtp.gmail.com:465"
)

type Mailer interface {
	SendMail(subject, content string, to, cc, bcc, attachFiles []string) error
}

type GMailer struct {
	name              string
	fromEmailAddress  string
	fromEmailPassword string
}

func NewGMailer(name, address, password string) Mailer {
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
