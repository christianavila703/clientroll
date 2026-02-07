# Page snapshot

```yaml
- generic [ref=e4]:
  - generic [ref=e5]:
    - generic [ref=e6]: C
    - paragraph [ref=e7]: Sign in to ClientRoll
  - generic [ref=e8]:
    - generic [ref=e9]:
      - generic [ref=e10]:
        - generic [ref=e11]: Email
        - textbox "Email" [ref=e12]:
          - /placeholder: you@company.com
      - generic [ref=e13]:
        - generic [ref=e14]: Password
        - generic [ref=e15]:
          - textbox "Password" [ref=e16]:
            - /placeholder: Enter password
          - button "Show" [ref=e17] [cursor=pointer]
      - button "Sign in" [ref=e18] [cursor=pointer]
    - link "Forgot password?" [ref=e20] [cursor=pointer]:
      - /url: /forgot-password
  - paragraph [ref=e21]:
    - text: Don't have an account?
    - link "Create one" [ref=e22] [cursor=pointer]:
      - /url: /register
  - button "Platform admin →" [ref=e24] [cursor=pointer]
```