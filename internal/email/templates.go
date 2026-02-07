package email

import "fmt"

// VerifyEmail generates the email verification email sent during registration.
func VerifyEmail(firstName, verifyURL string) SendInput {
	subject := "Verify your email — ClientRoll"

	html := fmt.Sprintf(`
<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:560px;margin:0 auto;padding:40px 20px;">
  <div style="text-align:center;margin-bottom:32px;">
    <div style="display:inline-block;background:#4f46e5;color:#fff;font-weight:700;font-size:14px;width:36px;height:36px;line-height:36px;border-radius:8px;">C</div>
  </div>
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Verify your email</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, please confirm your email address by clicking the button below.
    This link expires in 24 hours.
  </p>
  <div style="margin:24px 0;">
    <a href="%s" style="display:inline-block;background:#4f46e5;color:#fff;font-size:14px;font-weight:500;padding:10px 24px;border-radius:6px;text-decoration:none;">Verify Email</a>
  </div>
  <p style="font-size:13px;color:#9ca3af;line-height:1.5;margin:24px 0 0;">
    If you didn't create a ClientRoll account, you can safely ignore this email.
  </p>
  <hr style="border:none;border-top:1px solid #f3f4f6;margin:32px 0 16px;" />
  <p style="font-size:11px;color:#d1d5db;margin:0;">ClientRoll — Membership Management Platform</p>
</div>`, firstName, verifyURL)

	text := fmt.Sprintf("Verify your email\n\nHi %s, please confirm your email by visiting:\n%s\n\nThis link expires in 24 hours.\n\n— ClientRoll",
		firstName, verifyURL)

	return SendInput{Subject: subject, HTML: html, Text: text}
}

// WelcomeEmail generates the welcome email after tenant registration.
func WelcomeEmail(firstName, companyName, loginURL string) SendInput {
	subject := fmt.Sprintf("Welcome to ClientRoll, %s!", firstName)

	html := fmt.Sprintf(`
<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:560px;margin:0 auto;padding:40px 20px;">
  <div style="text-align:center;margin-bottom:32px;">
    <div style="display:inline-block;background:#4f46e5;color:#fff;font-weight:700;font-size:14px;width:36px;height:36px;line-height:36px;border-radius:8px;">C</div>
  </div>
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Welcome to ClientRoll</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, your account for <strong>%s</strong> has been created successfully.
    You can now start setting up your membership types, adding clients, and managing your business.
  </p>
  <div style="margin:24px 0;">
    <a href="%s" style="display:inline-block;background:#4f46e5;color:#fff;font-size:14px;font-weight:500;padding:10px 24px;border-radius:6px;text-decoration:none;">Go to Dashboard</a>
  </div>
  <p style="font-size:13px;color:#9ca3af;line-height:1.5;margin:24px 0 0;">
    Need help getting started? Reply to this email and we'll be happy to assist.
  </p>
  <hr style="border:none;border-top:1px solid #f3f4f6;margin:32px 0 16px;" />
  <p style="font-size:11px;color:#d1d5db;margin:0;">ClientRoll — Membership Management Platform</p>
</div>`, firstName, companyName, loginURL)

	text := fmt.Sprintf("Welcome to ClientRoll, %s!\n\nYour account for %s has been created.\n\nGo to your dashboard: %s\n\n— ClientRoll",
		firstName, companyName, loginURL)

	return SendInput{Subject: subject, HTML: html, Text: text}
}

// PasswordResetEmail generates the password reset email.
func PasswordResetEmail(firstName, resetURL string) SendInput {
	subject := "Reset your ClientRoll password"

	html := fmt.Sprintf(`
<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:560px;margin:0 auto;padding:40px 20px;">
  <div style="text-align:center;margin-bottom:32px;">
    <div style="display:inline-block;background:#4f46e5;color:#fff;font-weight:700;font-size:14px;width:36px;height:36px;line-height:36px;border-radius:8px;">C</div>
  </div>
  <h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Reset your password</h1>
  <p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 24px;">
    Hi %s, we received a request to reset your password. Click the button below to choose a new one.
    This link expires in 1 hour.
  </p>
  <div style="margin:24px 0;">
    <a href="%s" style="display:inline-block;background:#4f46e5;color:#fff;font-size:14px;font-weight:500;padding:10px 24px;border-radius:6px;text-decoration:none;">Reset Password</a>
  </div>
  <p style="font-size:13px;color:#9ca3af;line-height:1.5;margin:24px 0 0;">
    If you didn't request this, you can safely ignore this email. Your password won't be changed.
  </p>
  <hr style="border:none;border-top:1px solid #f3f4f6;margin:32px 0 16px;" />
  <p style="font-size:11px;color:#d1d5db;margin:0;">ClientRoll — Membership Management Platform</p>
</div>`, firstName, resetURL)

	text := fmt.Sprintf("Reset your ClientRoll password\n\nHi %s, click this link to reset your password (expires in 1 hour):\n%s\n\nIf you didn't request this, ignore this email.\n\n— ClientRoll",
		firstName, resetURL)

	return SendInput{Subject: subject, HTML: html, Text: text}
}
