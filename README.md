# ruf

A (vibe coded) application to make calls.

## What it does

This application is a CLI tool to send calls to different platforms. Currently, it supports Slack.

## Usage

To see a list of all available commands and flags, run:

```bash
ruf --help
```

## Configuration

The application is configured using a YAML file located at `$XDG_CONFIG_HOME/ruf/config.yaml`. The following configuration options are available:

| Name | Description |
| --- | --- |
| `source.urls` | A list of URLs to fetch calls from. Remote (`https://...`), local (`file://...`) and git (`git://...`) URLs are supported. See the Git Sources section for more information. |
| `slack.app_token` | The Slack app token to use for sending calls. |
| `git.tokens` | A map of git providers to personal access tokens. Currently, only `github.com` is supported. |

### Example

```yaml
source:
  urls:
    - "https://example.com/announcements.yaml"
    - "file:///path/to/local/announcements.yaml"
    - "git://github.com/andrewhowdencom/ruf-example-announcements/tree/main/example.yaml"

slack:
  app:
    token: ""

git:
  tokens:
    github.com: "YOUR_GITHUB_TOKEN"
```

### Git Sources

The application supports fetching calls from Git repositories. The URL format is:

`git://<repository>/tree/<refspec>/<file-path>`

For example:

`git://github.com/andrewhowdencom/ruf-example-announcements/tree/main/example.yaml`

### Slack Configuration

To use the Slack integration, you'll need to create a Slack app and install it in your workspace. The app will need the following permissions:

- `channels:read`: To list public channels.
- `groups:read`: To list private channels.
- `chat:write`: To send messages.
- `im:write`: To send direct messages.
- `users:read.email`: To look up users by email.

## Call Format

The application expects the source YAML files to contain a top-level `calls` list. Optionally, a `campaign` can be specified. If a campaign is not specified, it will be derived from the filename.

Each call must have a list of `triggers` that determine when the call should be sent. The following trigger types are available:

- `scheduled_at`: A specific time to send the call.
- `cron`: A cron expression for recurring calls.
- `sequence` and `delta`: For event-driven call sequences.

### Example

```yaml
campaign:
  id: "my-campaign"
  name: "My Campaign"
calls:
- id: "unique-id-1"
  author: "author@example.com"
  subject: "Hello!"
  content: "Hello, world!"
  destinations:
    - type: "slack"
      to:
        - "C1234567890"
  triggers:
    - scheduled_at: "2025-01-01T12:00:00Z"
- id: "unique-id-2"
  subject: "Recurring hello!"
  content: "Hello, recurring world!"
  destinations:
    - type: "slack"
      to:
        - "C1234567890"
  triggers:
    - cron: "0 * * * *"
      recurring: true
```

## Event-Driven Call Sequences

In addition to scheduled and recurring calls, the application also supports event-driven call sequences. This feature allows you to define a sequence of calls that are triggered by a specific event.

To use this feature, you'll need to define a call with a trigger that has a `sequence` and a `delta`, and then create an `event` with a matching `sequence` and a `start_time`.

- `sequence`: A unique identifier for the sequence.
- `delta`: A duration string (e.g., "5m", "1h30m") that specifies when the call should be sent relative to the event's `start_time`.
- `events`: A new top-level list in your source YAML file that contains a list of events.

### Author Impersonation

When a `Call` includes an `author` email address, `ruf` will attempt to send the message on behalf of that user.

- **Slack**: The message will appear to come from the author, using their Slack profile name and picture. If the user
  is not found in Slack, the message will be sent by the default bot, with the author's email appended to the message
  body for attribution.
- **Email**: The application will first attempt to send the email with the `From` address set to the author's email.
  If the configured SMTP server rejects this (due to security policies like SPF/DKIM), it will fall back to sending
  from the default configured sender address, but will set the `Reply-To` header to the author's email.

### Example

```yaml
campaign:
  id: "product-launch"
  name: "Product Launch"
calls:
- id: "launch-announcement-1"
  subject: "We're live!"
  content: "Our new product is now live! Check it out at..."
  destinations:
    - type: "slack"
      to:
        - "#general"
  triggers:
    - sequence: "product-launch-sequence"
      delta: "5m"
- id: "launch-announcement-2"
  subject: "Don't miss out!"
  content: "In case you missed it, our new product is now live! Check it out at..."
  destinations:
    - type: "slack"
      to:
        - "#marketing"
  triggers:
    - sequence: "product-launch-sequence"
      delta: "1h"
events:
- sequence: "product-launch-sequence"
  start_time: "2025-01-01T12:00:00Z"
  destinations:
    - type: "email"
      to:
        - "all-hands@example.com"
```

In this example, the two calls with the `sequence` "product-launch-sequence" will be triggered by the event with the same `sequence`. The first call will be sent 5 minutes after the event's `start_time`, and the second call will be sent 1 hour after. The destinations from the calls and the event will be merged, so the first call will be sent to the "#general" Slack channel and to "all-hands@example.com", and the second call will be sent to the "#marketing" Slack channel and to "all-hands@example.com".

## Migrating from the Old Format

The application provides a `migrate` command to help you update your old YAML files to the new `triggers` format. To migrate from the v0 format to the v1 format, simply run:

```bash
ruf migrate v1 /path/to/your/file.yaml
```

The command will print the migrated YAML to the console.

## Listing Sent Calls

When you list the sent calls, you will see the following statuses:

| Status | Description |
| --- | --- |
| `sent` | The call has been successfully sent. |
| `deleted` | The call has been sent and then subsequently deleted. |

## Getting it

You can download the latest version of the application from the [GitHub Releases page](https://github.com/andrewhowdencom/ruf/releases).

## Development

This application has been almost entirely "vibe coded" with Google Jules & Gemini.

## Task Runner

This project uses [Taskfile](https://taskfile.dev/) as a task runner for common development tasks. To use it, you'll first need to install it.

### Installation

You can install Taskfile with the following command:

```bash
go install github.com/go-task/task/v3/cmd/task@latest
```

### Usage

To see a list of all available tasks, run:

```bash
task --list
```

You can then run any task with `task <task-name>`.

## Running as a Service

This project includes an example `systemd` unit file that can be used to run the application as a user-level service.

To install it, copy the file to `~/.config/systemd/user/`:

```bash
mkdir -p ~/.config/systemd/user
cp examples/ruf.service ~/.config/systemd/user/
```
