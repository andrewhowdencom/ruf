# ruf

A (vibe coded) application to make calls.

## What it does

This application is a CLI tool to send calls to different platforms. Currently, it supports Slack.

## Usage

To see a list of all available commands and flags, run:

```bash
ruf --help
```

## Persistent Scheduling

The application uses a persistent scheduling model. Instead of calculating the schedule on-the-fly, the scheduler pre-calculates all call instances within a defined time window and stores them in its datastore. The worker then reads from this persistent schedule to determine which calls to send.

The schedule is automatically recalculated whenever a change is detected in the source files. You can also manually trigger a refresh of the schedule by running the following command:

```bash
ruf scheduled refresh
```

This command will refetch all source files, recalculate the entire schedule, and update the datastore with the new information.

## Configuration

The application is configured using a YAML file located at `$XDG_CONFIG_HOME/ruf/config.yaml`.

An example, well-documented configuration file can be found at [`examples/config.yaml`](./examples/config.yaml).

### Time Slot Scheduling

This application supports a time slot scheduling feature that allows you to define specific time slots for your calls. If you enable this feature, any recurring calls, or calls scheduled at midnight, will be scheduled in the next available time slot.

Slots can be configured on a per-destination basis, with a fallback to a global default. The configuration is hierarchical, with the following order of precedence:

1.  **Destination-specific:** `slots.<type>.<destination>` (e.g., `slots.slack."#general"`)
2.  **Type-specific:** `slots.<type>.default` (e.g., `slots.slack.default`)
3.  **Global default:** `slots.default`

To configure the time slots, add a `slots` section to your `config.yaml` file. The following options are available:

- `timezone`: The timezone to use for the time slots. It should be a valid IANA Time Zone database name (e.g. "Europe/Berlin").

#### Example

```yaml
slots:
  timezone: "Europe/Berlin"
  default:
    monday:
      - "09:00"
      - "14:00"
  slack:
    default:
      monday:
        - "10:00"
    "#general":
      monday:
        - "11:00"
```

In this example:
- All non-Slack destinations will use the global default slots (9am and 2pm on Mondays).
- Slack destinations will use the Slack-specific default slots (10am on Mondays), unless a more specific configuration is provided.
- The `#general` Slack channel will use its own specific slot (11am on Mondays).

If you do not configure any time slots, the application will default to "09:00" and "14:00" for every day of the week, in UTC.

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
- `rrule`: An iCal `rrule` string for more complex recurring calls.
- `hijri`: A date in the Islamic (Hijri) calendar.
- `sequence` and `delta`: For event-driven call sequences.

**Note:** Recurring calls (cron and rrule) and calls scheduled at midnight will be scheduled using the time slot scheduling feature, if it is configured.

### Content Formatting

The `content` of a call can be written in Markdown. This will be automatically converted to the appropriate format for the destination. For example, it will be converted to HTML for email and Slack's `mrkdwn` for Slack.

### Example

For a detailed example of a calls file, see [`examples/calls.yaml`](./examples/calls.yaml).

For an example of scheduling calls based on the Islamic (Hijri) calendar, see [`examples/hijri_schedule.yaml`](./examples/hijri_schedule.yaml).

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

## Deploying to Google Cloud Run

This application can be deployed to Google Cloud Run. The following instructions assume you have the `gcloud` CLI installed and configured.

### 1. Enable Required Services

First, you'll need to enable the Cloud Run, Artifact Registry, and Cloud Build services:

```bash
gcloud services enable run.googleapis.com artifactregistry.googleapis.com cloudbuild.googleapis.com
```

### 2. Create an Artifact Registry Repository

Next, create an Artifact Registry repository to store the Docker images:

```bash
gcloud artifacts repositories create ruf \
  --repository-format=docker \
  --location=europe-west3
```

### 3. Create a Service Account

Create a service account for the Cloud Run service to use:

```bash
gcloud iam service-accounts create ruf-runner
```

### 4. Grant Permissions

Grant the service account the necessary permissions to access Firestore and pull images:

```bash
gcloud projects add-iam-policy-binding <YOUR_PROJECT_ID> \
  --member="serviceAccount:ruf-runner@<YOUR_PROJECT_ID>.iam.gserviceaccount.com" \
  --role="roles/datastore.user"

gcloud projects add-iam-policy-binding <YOUR_PROJECT_ID> \
  --member="serviceAccount:ruf-runner@<YOUR_PROJECT_ID>.iam.gserviceaccount.com" \
  --role="roles/artifactregistry.reader"
```

### 5. Set up Continuous Deployment with GitHub Actions

This repository includes a GitHub Actions workflow to automatically deploy the application to Cloud Run. To use it, you'll need to set up Workload Identity Federation to allow GitHub Actions to securely authenticate with Google Cloud.

#### 5.1. Create a Service Account for GitHub Actions

First, create a service account that GitHub Actions will use to deploy the application:

```bash
gcloud iam service-accounts create github-actions-runner \
  --display-name="GitHub Actions Runner"
```

#### 5.2. Grant Permissions to the Service Account

Grant the service account the necessary permissions to deploy to Cloud Run and push to Artifact Registry:

```bash
gcloud projects add-iam-policy-binding <YOUR_PROJECT_ID> \
  --member="serviceAccount:github-actions-runner@<YOUR_PROJECT_ID>.iam.gserviceaccount.com" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding <YOUR_PROJECT_ID> \
  --member="serviceAccount:github-actions-runner@<YOUR_PROJECT_ID>.iam.gserviceaccount.com" \
  --role="roles/artifactregistry.writer"

gcloud projects add-iam-policy-binding <YOUR_PROJECT_ID> \
  --member="serviceAccount:github-actions-runner@<YOUR_PROJECT_ID>.iam.gserviceaccount.com" \
  --role="roles/iam.serviceAccountUser"
```

Replace `<YOUR_PROJECT_ID>` with your Google Cloud project ID.

#### 5.3. Create a Workload Identity Pool and Provider

Next, create a Workload Identity Pool and a Provider to allow GitHub Actions to authenticate:

```bash
gcloud iam workload-identity-pools create github-actions-pool \
  --location="global" \
  --display-name="GitHub Actions Pool"

gcloud iam workload-identity-pools providers create-oidc github-actions-provider \
  --workload-identity-pool="github-actions-pool" \
  --location="global" \
  --issuer-uri="https://token.actions.githubusercontent.com" \
  --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository"
```

#### 5.4. Allow Authentications from the Provider

Allow the GitHub Actions service account to be impersonated by the Workload Identity Provider:

```bash
gcloud iam service-accounts add-iam-policy-binding "github-actions-runner@<YOUR_PROJECT_ID>.iam.gserviceaccount.com" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/<PROJECT_NUMBER>/locations/global/workloadIdentityPools/github-actions-pool/subject/repo:<YOUR_GITHUB_ORG>/<YOUR_GITHUB_REPO>:ref:refs/heads/main"
```

Replace `<YOUR_PROJECT_ID>` with your Google Cloud project ID, `<PROJECT_NUMBER>` with your Google Cloud project number, and `<YOUR_GITHUB_ORG>/<YOUR_GITHUB_REPO>` with your GitHub organization and repository.

#### 5.5. Create GitHub Secrets

Finally, create the following secrets in your GitHub repository:

- `GCP_PROJECT_ID`: Your Google Cloud project ID.
- `GCP_WORKLOAD_IDENTITY_PROVIDER`: The full identifier of the Workload Identity Provider. You can get this by running the following command:
  ```bash
  gcloud iam workload-identity-pools providers describe github-actions-provider \
    --workload-identity-pool="github-actions-pool" \
    --location="global" \
    --format="value(name)"
  ```
- `GCP_SERVICE_ACCOUNT_EMAIL`: The email address of the `github-actions-runner` service account.

Once you've created these secrets, the GitHub Actions workflow will automatically build and deploy the application to Cloud Run when changes are pushed to the `main` branch.
