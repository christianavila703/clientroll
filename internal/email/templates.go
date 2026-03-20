package email

import "fmt"

const emailWrapper = `<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:560px;margin:0 auto;padding:40px 20px;">
  <div style="text-align:center;margin-bottom:32px;">
    <div style="display:inline-block;background:#4f46e5;color:#fff;font-weight:700;font-size:14px;width:36px;height:36px;line-height:36px;border-radius:8px;">C</div>
  </div>
  %s
  <hr style="border:none;border-top:1px solid #f3f4f6;margin:32px 0 16px;" />
  <p style="font-size:11px;color:#d1d5db;margin:0;">ClientRoll — Membership Management Platform</p>
</div>`

func wrap(body string) string {
	return fmt.Sprintf(emailWrapper, body)
}

// WrapHTML wraps a HTML fragment in the standard email shell.
func WrapHTML(body string) string {
	return wrap(body)
}

// VerifyEmail — sent during tenant registration.
func VerifyEmail(firstName, verifyURL string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Verify your email</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, please confirm your email address by clicking the button below. This link expires in 24 hours.
  </p>
  <div style="margin:24px 0;">
    <a href="%s" style="display:inline-block;background:#4f46e5;color:#fff;font-size:14px;font-weight:500;padding:10px 24px;border-radius:6px;text-decoration:none;">Verify Email</a>
  </div>
  <p style="font-size:13px;color:#9ca3af;line-height:1.5;margin:24px 0 0;">If you didn't create a ClientRoll account, you can safely ignore this email.</p>`,
		firstName, verifyURL))

	return SendInput{
		Subject: "Verify your email — ClientRoll",
		HTML:    html,
		Text:    fmt.Sprintf("Hi %s, verify your email: %s", firstName, verifyURL),
	}
}

// WelcomeEmail — sent after email verification / tenant setup.
func WelcomeEmail(firstName, companyName, loginURL string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Welcome to ClientRoll</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, your account for <strong>%s</strong> has been created successfully.
    You can now start setting up your membership types, adding clients, and managing your business.
  </p>
  <div style="margin:24px 0;">
    <a href="%s" style="display:inline-block;background:#4f46e5;color:#fff;font-size:14px;font-weight:500;padding:10px 24px;border-radius:6px;text-decoration:none;">Go to Dashboard</a>
  </div>`,
		firstName, companyName, loginURL))

	return SendInput{
		Subject: fmt.Sprintf("Welcome to ClientRoll, %s!", firstName),
		HTML:    html,
		Text:    fmt.Sprintf("Welcome to ClientRoll, %s! Your account for %s is ready: %s", firstName, companyName, loginURL),
	}
}

// PasswordResetEmail — sent when user requests a password reset.
func PasswordResetEmail(firstName, resetURL string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Reset your password</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, we received a request to reset your password. Click the button below to choose a new one. This link expires in 1 hour.
  </p>
  <div style="margin:24px 0;">
    <a href="%s" style="display:inline-block;background:#4f46e5;color:#fff;font-size:14px;font-weight:500;padding:10px 24px;border-radius:6px;text-decoration:none;">Reset Password</a>
  </div>
  <p style="font-size:13px;color:#9ca3af;line-height:1.5;margin:24px 0 0;">If you didn't request this, you can safely ignore this email.</p>`,
		firstName, resetURL))

	return SendInput{
		Subject: "Reset your ClientRoll password",
		HTML:    html,
		Text:    fmt.Sprintf("Hi %s, reset your password: %s", firstName, resetURL),
	}
}

// MembershipConfirmationEmail — sent to client after membership signup.
func MembershipConfirmationEmail(clientName, membershipTitle string, termMonths int, totalFee float64, companyName string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Membership Confirmed</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, your membership has been successfully set up. Here are your details:
  </p>
  <div style="background:#f9fafb;border:1px solid #e5e7eb;border-radius:8px;padding:16px;margin:0 0 24px;">
    <table style="width:100%%;font-size:14px;color:#374151;">
      <tr><td style="padding:6px 0;color:#6b7280;">Membership</td><td style="padding:6px 0;font-weight:600;text-align:right;">%s</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Term</td><td style="padding:6px 0;font-weight:600;text-align:right;">%d months</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Total Fee</td><td style="padding:6px 0;font-weight:600;text-align:right;">$%.2f</td></tr>
    </table>
  </div>
  <p style="font-size:13px;color:#9ca3af;line-height:1.5;margin:0;">If you have any questions, please contact %s directly.</p>`,
		clientName, membershipTitle, termMonths, totalFee, companyName))

	return SendInput{
		Subject: fmt.Sprintf("Your %s membership is confirmed", membershipTitle),
		HTML:    html,
		Text:    fmt.Sprintf("Hi %s, your %s membership (%d months, $%.2f) has been confirmed.", clientName, membershipTitle, termMonths, totalFee),
	}
}

// StaffInvitationEmail — sent when a tenant admin invites a new staff member.
func StaffInvitationEmail(inviteeName, inviterName, companyName, inviteURL string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">You're invited to join %s</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi%s, %s has invited you to join <strong>%s</strong> on ClientRoll. Click below to set up your account. This link expires in 48 hours.
  </p>
  <div style="margin:24px 0;">
    <a href="%s" style="display:inline-block;background:#4f46e5;color:#fff;font-size:14px;font-weight:500;padding:10px 24px;border-radius:6px;text-decoration:none;">Accept Invitation</a>
  </div>`,
		companyName,
		func() string { if inviteeName != "" { return " " + inviteeName } ; return "" }(),
		inviterName, companyName, inviteURL))

	return SendInput{
		Subject: fmt.Sprintf("You're invited to join %s on ClientRoll", companyName),
		HTML:    html,
		Text:    fmt.Sprintf("You've been invited to join %s on ClientRoll by %s. Accept: %s", companyName, inviterName, inviteURL),
	}
}

// PaymentReceiptEmail — sent to client after a successful payment.
func PaymentReceiptEmail(clientName string, amount float64, membershipTitle, paymentDate, companyName string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Payment Receipt</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, we've received your payment. Here's your receipt:
  </p>
  <div style="background:#f9fafb;border:1px solid #e5e7eb;border-radius:8px;padding:16px;margin:0 0 24px;">
    <table style="width:100%%;font-size:14px;color:#374151;">
      <tr><td style="padding:6px 0;color:#6b7280;">Amount</td><td style="padding:6px 0;font-weight:600;text-align:right;">$%.2f</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Membership</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Date</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
    </table>
  </div>
  <p style="font-size:13px;color:#9ca3af;line-height:1.5;margin:0;">Thank you for your payment. — %s</p>`,
		clientName, amount, membershipTitle, paymentDate, companyName))

	return SendInput{
		Subject: fmt.Sprintf("Payment receipt — $%.2f", amount),
		HTML:    html,
		Text:    fmt.Sprintf("Hi %s, payment of $%.2f for %s received on %s.", clientName, amount, membershipTitle, paymentDate),
	}
}

// PaymentFailedEmail — sent to client when a payment fails.
func PaymentFailedEmail(clientName string, amount float64, membershipTitle, failureReason, companyName string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Payment Failed</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, unfortunately your scheduled payment could not be processed.
  </p>
  <div style="background:#fef2f2;border:1px solid #fecaca;border-radius:8px;padding:16px;margin:0 0 24px;">
    <table style="width:100%%;font-size:14px;color:#374151;">
      <tr><td style="padding:6px 0;color:#6b7280;">Amount</td><td style="padding:6px 0;font-weight:600;text-align:right;">$%.2f</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Membership</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Reason</td><td style="padding:6px 0;text-align:right;color:#dc2626;">%s</td></tr>
    </table>
  </div>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 16px;">
    Please ensure your bank details are up to date. We'll automatically retry this payment. If you have questions, please contact %s.
  </p>`,
		clientName, amount, membershipTitle, failureReason, companyName))

	return SendInput{
		Subject: "Payment failed — action required",
		HTML:    html,
		Text:    fmt.Sprintf("Hi %s, your payment of $%.2f for %s failed: %s. Please check your bank details.", clientName, amount, membershipTitle, failureReason),
	}
}

// PaymentFailedAlertEmail — sent to tenant admin when a client's payment fails.
func PaymentFailedAlertEmail(adminName, clientName string, amount float64, membershipTitle, failureReason string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Payment Failed Alert</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, a payment has failed for one of your clients:
  </p>
  <div style="background:#fef2f2;border:1px solid #fecaca;border-radius:8px;padding:16px;margin:0 0 24px;">
    <table style="width:100%%;font-size:14px;color:#374151;">
      <tr><td style="padding:6px 0;color:#6b7280;">Client</td><td style="padding:6px 0;font-weight:600;text-align:right;">%s</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Amount</td><td style="padding:6px 0;text-align:right;">$%.2f</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Membership</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Reason</td><td style="padding:6px 0;text-align:right;color:#dc2626;">%s</td></tr>
    </table>
  </div>
  <p style="font-size:13px;color:#9ca3af;line-height:1.5;margin:0;">Review this in your ClientRoll dashboard under Payments.</p>`,
		adminName, clientName, amount, membershipTitle, failureReason))

	return SendInput{
		Subject: fmt.Sprintf("Payment failed: %s — $%.2f", clientName, amount),
		HTML:    html,
		Text:    fmt.Sprintf("Payment failed for %s: $%.2f for %s. Reason: %s", clientName, amount, membershipTitle, failureReason),
	}
}

// DisputeAlertEmail — sent to tenant admin when a payment is disputed.
func DisputeAlertEmail(adminName, clientName string, amount float64, reason string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Payment Dispute Opened</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, a payment dispute has been opened:
  </p>
  <div style="background:#fffbeb;border:1px solid #fde68a;border-radius:8px;padding:16px;margin:0 0 24px;">
    <table style="width:100%%;font-size:14px;color:#374151;">
      <tr><td style="padding:6px 0;color:#6b7280;">Client</td><td style="padding:6px 0;font-weight:600;text-align:right;">%s</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Amount</td><td style="padding:6px 0;text-align:right;">$%.2f</td></tr>
      <tr><td style="padding:6px 0;color:#6b7280;">Reason</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
    </table>
  </div>
  <p style="font-size:13px;color:#9ca3af;line-height:1.5;margin:0;">Please review and respond to this dispute in your ClientRoll dashboard.</p>`,
		adminName, clientName, amount, reason))

	return SendInput{
		Subject: fmt.Sprintf("Dispute opened: %s — $%.2f", clientName, amount),
		HTML:    html,
		Text:    fmt.Sprintf("Dispute opened by %s for $%.2f. Reason: %s", clientName, amount, reason),
	}
}

// MembershipAgreementEmail — sent to client with signed agreement PDF attached.
func MembershipAgreementEmail(clientName, membershipTitle, companyName string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Your Membership Agreement</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, please find your signed membership agreement for <strong>%s</strong> with <strong>%s</strong> attached to this email as a PDF.
  </p>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Please save this document for your records. It contains a complete copy of your signed agreement including
    all terms and conditions, your authorisation signature, and membership details.
  </p>
  <p style="font-size:13px;color:#9ca3af;line-height:1.5;margin:0;">If you have any questions about your membership, please contact %s directly.</p>`,
		clientName, membershipTitle, companyName, companyName))

	return SendInput{
		Subject: fmt.Sprintf("Your signed membership agreement — %s", membershipTitle),
		HTML:    html,
		Text:    fmt.Sprintf("Hi %s, your signed membership agreement for %s with %s is attached as a PDF.", clientName, membershipTitle, companyName),
	}
}

// MembershipExpiringEmail — sent to client when membership is nearing end of term.
func MembershipExpiringEmail(clientName, membershipTitle, expiryDate, companyName string) SendInput {
	html := wrap(fmt.Sprintf(`
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Your Membership is Expiring Soon</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, your <strong>%s</strong> membership is set to expire on <strong>%s</strong>.
  </p>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Please contact %s if you'd like to renew or discuss your options.
  </p>`,
		clientName, membershipTitle, expiryDate, companyName))

	return SendInput{
		Subject: fmt.Sprintf("Your %s membership expires soon", membershipTitle),
		HTML:    html,
		Text:    fmt.Sprintf("Hi %s, your %s membership expires on %s. Contact %s to renew.", clientName, membershipTitle, expiryDate, companyName),
	}
}
