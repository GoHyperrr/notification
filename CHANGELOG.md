# Changelog

All notable changes to this project will be documented in this file.

## [0.1.0] - 2026-06-14

### Added
- Created standalone, top-level Go module `github.com/GoHyperrr/notification`.
- Integrated standard `net/smtp` provider with support for multiple sender profiles (e.g. orders, support).
- Built local mock SMTP TCP test suite verifying message dispatch and content routing.
- Integrated routing capability with multi-channel support (SMTP email & Twilio WhatsApp).
