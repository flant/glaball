[![Go](https://github.com/flant/glaball/actions/workflows/go.yml/badge.svg)](https://github.com/flant/glaball/actions/workflows/go.yml)
[![Docker Image](https://github.com/flant/glaball/actions/workflows/docker.yml/badge.svg)](https://github.com/flant/glaball/actions/workflows/docker.yml)

Glaball is a CLI tool to manage multiple self-hosted GitLab instances at the same time. [This announcement](https://blog.flant.com/glaball-to-manage-gitlab-instances-in-bulk/) tells the story why it was created.

**Contents**:
* [Features](#features)
* [Installing](#installing)
  * [Building](#building)
  * [Configuring](#configuring)
  * [How to add a GitLab host?](#how-to-add-a-gitlab-host)
  * [Important note on security](#important-note-on-security)
* [Usage](#usage)
  * [Autocompletion](#autocompletion)
  * [Usage examples](#usage-examples)
* [Community](#community)
* [License](#license)

## Features

Glaball currently supports the following features:

* Creating/blocking/deleting/modifying users (`users [create|block|delete|modify]`)
* Displaying a list of users with sorting/grouping/filtering options (`users list`)
* Searching for a specific user (`users search`)
* Displaying a list of repositories with sorting/grouping/filtering options (`projects list`)
* Editing some repository options (`projects edit`)
* Searching for scheduled jobs in repositories and filtering by active/inactive status (`projects pipelines schedules`)
* Searching for the regex pattern in the specified files in the repositories (`projects files search`)
* Displaying a list of current GitLab instance versions with information on whether an update is necessary (up to date|update available|update asap)
* Displaying information about the current API user (`whoami`)

## Installing

You can get the compiled binary at [the releases page](https://github.com/flant/glaball/releases).

### Building

```
$ git clone https://github.com/flant/glaball.git
$ cd glaball
$ go build -v -o build/glaball *.go
```

### Configuring

```
$ cat ~/.config/glaball/config.yaml

# Cache settings
cache:
  # HTTP request cache is enabled by default
  enabled: true
  # The cache is stored in the user's cache directory by default
  # $HOME/.cache/glaball
  path: ""
  # The default cache size is 100MB
  size: 100MB
  # GZIP cache compression is enabled by default
  compression: true
  # By default, the cache is valid for 1 day
  ttl: 24h

# By default, operations are performed for all hosts.
# You can use a regexp command/project filter (e.g., "main.*" or "main.example-project")
# at the config level or use the --filter flag (-f)
filter: ".*"

# By default, count of hosts in grouped output is limited to 5.
# If you want to show all hosts, set this option to true or use the --all flag (-a).
all: false

# The number of simultaneous HTTP connections
threads: 100

# Host list (required)
# The project name is generated as follows: "<team>.<project>.<name>"
hosts:
  # The team (e.g., main)
  team:
    # The project name (e.g., example-project)
    project:
      # The GitLab name
      name:
        # Link to the project's GitLab repo
        url: https://gitlab.example.com
        # Custom IP address of host
        # If you want to override IP address resolved by the default resolver, use this option.
        ip: 127.0.0.1
        # The user's token the client will use to connect to the API
        # API token - provides full permissions, including user creation/deletion
        # Read API token - provides read-only access without permissions to create/modify users or filtering the list of users by their email address
        token: <api|read_api token>
        # Rate limit check
        # This one is disabled by default if a cache is used because it generates additional non-cacheable requests.
        # It only makes sense to enable it if the rate limit in GitLab is enabled
        # https://docs.gitlab.com/ee/security/rate_limits.html
        rate_limit: false
```

### How to add a GitLab host?
- Go to https://gitlab.example.com/-/profile/personal_access_tokens as a user;
- Create a **Personal Access Token** named `glaball` with the following scope:
  - `read_api` - if only read access is required;
  - `api` - if full access is required. This one allows you to create users and so forth (note that the user must have the admin privileges);
- Add your host and a token to the `config.yaml` file and copy it to the glaball directory (or specify the path to the config file via the `--config=path` parameter).

Example:
```
hosts:
  main:
    example-project:
      primary:
        url: https://gitlab-primary.example.com
        token: api_token
      secondary:
        url: https://gitlab-secondary.example.com
        token: api_token
```

Let's check that the host is in the config:
```
$ glaball config list
[main.example-project.secondary] https://gitlab-secondary.example.com
[main.example-project.primary]   https://gitlab-primary.example.com
Total: 2
```

### Important note on security

Currently, all access tokens are simply stored in the glaball configuration file from where they can be used to access relevant GitLab instances. Remember to set appropriate permissions on your config, use `read_api` tokens only (if possible), and apply a strict expiry policy for your tokens.

## Usage

```
$ glaball -h
Gitlab bulk administration tool

Usage:
  glaball [flags]
  glaball [command]

Available Commands:
  cache       Cache management
  completion  Generate the autocompletion script for the specified shell
  config      Information about the current configuration
  help        Help about any command
  info        Information about the current build
  projects    Projects API
  users       Users API
  versions    Retrieve version information for GitLab instances
  whoami      Current API user

Flags:
  -a, --all                Show all hosts in grouped output
      --config string      Path to the configuration file. (default "$HOME/.config/glaball/config.yaml")
  -f, --filter string      Select Gitlab(s) by regexp filter (default ".*")
  -h, --help               help for glaball
      --log_level string   Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, off] (default "info")
      --threads int        Number of concurrent processes. (default: one process for each Gitlab instances in config file) (default 100)
      --ttl duration       Override cache TTL set in config file (default 24h0m0s)
  -u, --update             Refresh cache
  -v, --verbose            Verbose output

Use "glaball [command] --help" for more information about a command.
```

### Autocompletion

#### Bash
```
$ glaball completion bash -h

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:
$ source <(glaball completion bash)

To load completions for every new session, execute once:
Linux:
  $ glaball completion bash > /etc/bash_completion.d/glaball
MacOS:
  $ glaball completion bash > /usr/local/etc/bash_completion.d/glaball

You will need to start a new shell for this setup to take effect.
```

#### Zsh

```
$ glaball completion zsh -h

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

$ echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions for every new session, execute once:
# Linux:
$ glaball completion zsh > "${fpath[1]}/_glaball"
# macOS:
$ glaball completion zsh > /usr/local/share/zsh/site-functions/_glaball

You will need to start a new shell for this setup to take effect.
```

## Usage examples

### Create a user
Here is how you can create a new user in `main` team projects:

```
$ glaball users create --filter "main.*" --email=test@test.com --username=test-glaball --name="test glaball" --password="qwerty"
```

### Block a user
```
$ glaball users block --by=username test-glaball
```

Display the list of projects in which this user exists:
```
$ glaball users block --by=username test-glaball --hosts
```

### Edit projects options
```
$ glaball projects edit --search_namespaces=true --search mygroup/ --ci_forward_deployment_enabled=false
```

### List opened merge requests
```
$ glaball projects mr list
```

### Protect main branch in all projects
```
$ glaball projects branches protected protect --name="main" --merge_access_level=30
```

### Search for a pattern in the files
*You can search through several files at once, but that (at least) doubles the search time.*

*You can search for several patterns at once; it does not affect the search time.*

```
$ glaball projects files search --filepath="werf.yaml,werf.yml" --pattern="mount" --show
```

### Search for scheduled pipelines
The command below displays a list of projects with inactive scheduled pipelines:
```
$ glaball -f "main.*" projects pipelines schedules --active=false
```

### List and create [werf](https://github.com/werf/werf) cleanup [schedules](https://werf.io/documentation/v1.2/advanced/ci_cd/gitlab_ci_cd.html#cleaning-up-images)
List all cleanup schedules including non-existent:
```
$ glaball projects pipelines cleanups
```

Create cleanup schedules in all projects (only **one** gitlab host):
```
$ glaball projects pipelines cleanups -f "main.example-project.primary" --create --setowner ${WERF_IMAGES_CLEANUP_PASSWORD}
```

Change the owner of existing cleanup scheduled pipelines (only **one** gitlab host):
```
$ glaball projects pipelines cleanups -f "main.example-project.primary" --setowner ${WERF_IMAGES_CLEANUP_PASSWORD}
```

### Get information about the projects' container registries
```
$ glaball projects registry list --size | sort -k 4 -h
```

### Search for a user
```
$ glaball users search --by=username docker
```

### List users
Display the list of users grouped by the username:
```
$ glaball users list --group_by=username
```

Show only those users who are active in *n* projects:
```
$ glaball users list --group_by=username --count n --all
```

Display the list of administrators:
```
$ glaball users list --group_by=username --admins=true
```

### Show the list of hosts specified in the config
```
$ glaball config list
```

### Clear the cache
```
$ glaball cache clean
```

### Show the list of current versions
```
$ glaball versions
```

### Show the active user
```
$ glaball whoami
```

# Community

Originally created in [Flant](https://flant.com/).

Please, feel free to reach developers/maintainers and users via [GitHub Discussions](https://github.com/flant/glaball/discussions) for any questions.

You're welcome to follow [@flant_com](https://twitter.com/flant_com) to stay informed about all our Open Source initiatives.

# License

Apache License 2.0, see [LICENSE](LICENSE).

GITLAB, the GitLab logo and all other GitLab trademarks herein are the registered and unregistered trademarks of [GitLab, Inc.](https://about.gitlab.com/)
