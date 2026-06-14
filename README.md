# notification — Hyperrr Pluggable Notification Module

[![Go Reference](https://pkg.go.dev/badge/github.com/GoHyperrr/notification.svg)](https://pkg.go.dev/github.com/GoHyperrr/notification)

This repository contains the standalone, pluggable notification and messaging providers for the Hyperrr engine. It includes support for SMTP email sending, Twilio WhatsApp integration, multi-channel routing, and multiple sender profiles (e.g. `orders@mango.in`, `support@mango.in`).

---

## 📬 Features

* **SMTP Email Provider**: Sends HTML and plain-text emails via standard `net/smtp`.
* **Multi-profile Sender Support**: Dynamically routes messages using different sender profiles and credentials depending on context (e.g., support vs. sales vs. transactional notifications).
* **Twilio WhatsApp Provider**: Integrates with Twilio for WhatsApp messaging campaigns and notifications.
* **Multi-Channel Router**: Intelligently routes notifications to email, WhatsApp, or both based on preferences and recipient endpoints.
* **Event-Native Workflows**: Reacts automatically to platform events (such as user registrations or order completion status) via the Hyperrr event bus.

---

## 🛠️ Usage

This module is imported by the core Hyperrr engine and registers its notification models, routes, and step handlers during runtime initialization.

To learn more about how to configure notification profiles or build custom channels, see the [Hyperrr Developer Guide](https://github.com/GoHyperrr/hyperrr/blob/main/developer.md).
