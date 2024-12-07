# Changelog

## v0.2.1

### Refactor

- Server add sefl-notify manager
- Collector loop and initialization refactoring
- Improved IP Tracker

## v0.2.0

Architectural refactor and feature enhancements.

### Breaking Changes

- Restructured to server-agent architecture
- Changed configuration format to support new architecture
- Updated notification system to support both standalone and server modes

### Features

- Server-agent architecture for centralized monitoring
- Extended notification channels:
  - Email (enhanced)
  - Telegram (enhanced)
  - Slack
  - Discord
  - DingTalk
  - WeChat Work
  - Webhook
- Multiple database backends:
  - SQLite
  - MySQL
  - PostgreSQL
- RESTful API for metrics collection and monitoring
- Docker support for both server and agent
- Systemd and Launchd service configurations

## v0.1.0

Initial release with basic IP monitoring functionality.

### Features

- IP change monitoring
- Basic notification support:
  - Email
  - Telegram
- Standalone operation
