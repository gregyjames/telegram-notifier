[![Docker](https://github.com/gregyjames/telegram-notifier/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/gregyjames/telegram-notifier/actions/workflows/docker-publish.yml)
![GitHub repo size](https://img.shields.io/github/repo-size/gregyjames/telegram-notifier)

# telegram-notifier
A simple, self-hosted REST server for Raspberry Pi that sends Telegram notifications.

## How to use
### Docker compose

```yaml
services:
  telegram:
  container_name: telegram_notifier
  image: 'ghcr.io/gregyjames/telegram-notifier:main'
  restart: unless-stopped
  ports:
    - "8080:8080"
  volumes:
    - ./telegram:/usr/src/app/data
```
###  Create config
In the same directory as your Compose file, create a new folder named `telegram` and create a `config.json` file with the following contents:
```json
{
	"key": "Telegram API KEY from BotFather",
	"chatid": "Telegram ChatID"
}
```
### Run
`docker compose up -d`

### Send message
`curl "http://SERVERIP:8080/get?message=hello"`

## Features
- Fast
- Lightweight (1.85MB!)
- Quick Setup

## License
MIT License

Copyright (c) 2024 Greg James

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
