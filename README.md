[![Go](https://github.com/flant/gitlaball/actions/workflows/go.yml/badge.svg)](https://github.com/flant/gitlaball/actions/workflows/go.yml)
[![Docker Image](https://github.com/flant/gitlaball/actions/workflows/docker.yml/badge.svg)](https://github.com/flant/gitlaball/actions/workflows/docker.yml)

# A tool for administering self-hosted GitLab instances

You can get the compiled binary from the release page at https://github.com/flant/github.com/flant/gitlaball/releases

Building:
```
$ git clone https://github.com/flant/gitlaball.git
$ cd gitlaball
$ go build -v -o build/gitlaball *.go
```

## Configuring

```
$ cat ~/.config/gitlaball/config.yaml

# Cache settings
cache:
  # HTTP request cache is enabled by default
  enabled: true
  # The cache is stored in the user's cache directory by default
  # $HOME/.cache/gitlaball
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

### How do I add a host?
- Go to https://gitlab.example.com/-/profile/personal_access_tokens as a user;
- Create a **Personal Access Token** named `gitlaball` with the following scope:
  - `read_api` - if only read access is required;
  - `api` - if full access is required. This one allows you to create users and so forth (note that the user must have the admin privileges);
- Add your host and a token to the `config.yaml` file and copy it to the gitlaball directory (or specify the path to the config file via the `--config=path` parameter).

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
$ gitlaball config list
[main.example-project.secondary] https://gitlab-secondary.example.com
[main.example-project.primary]   https://gitlab-primary.example.com
Total: 2
```

## Usage

Gitlaball currently supports the following features:

* Creating/blocking/deleting/modifying users (`users [create|block|delete|modify]`)
* Displaying a list of users with sorting/grouping/filtering options (`users list`)
* Searching for a specific user (`users search`)
* Displaying a list of repositories with sorting/grouping/filtering options (`projects list`)
* Searching for scheduled jobs in repositories and filtering by active/inactive status (`projects pipelines schedules`)
* Searching for the regex pattern in the specified files in the repositories (`projects files search`)
* Displaying a list of current GitLab instance versions with information on whether an update is necessary (up to date|update available|update asap)
* Displaying information about the current API user (`whoami`)

```
$ gitlaball -h
Gitlab bulk administration tool

Usage:
  gitlaball [flags]
  gitlaball [command]

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
      --config string      Path to the configuration file. (default "$HOME/.config/gitlaball/config.yaml")
  -f, --filter string      Select Gitlab(s) by regexp filter (default ".*")
  -h, --help               help for gitlaball
      --log_level string   Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, off] (default "info")
      --threads int        Number of concurrent processes. (default: one process for each Gitlab instances in config file) (default 100)
      --ttl duration       Override cache TTL set in config file (default 24h0m0s)
  -u, --update             Refresh cache
  -v, --verbose            Verbose output

Use "gitlaball [command] --help" for more information about a command.
```

### Autocompletion

#### Bash
```
$ gitlaball completion bash -h

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

This command loads a list of completions in your current shell session:
$ source <(gitlaball completion bash)

Run these to load completions for every new session:
Linux:
  $ gitlaball completion bash > /etc/bash_completion.d/gitlaball
MacOS:
  $ gitlaball completion bash > /usr/local/etc/bash_completion.d/gitlaball

Start a new shell for this setup to take effect.
```

#### Zsh

```
$ gitlaball completion zsh -h

Generate the autocompletion script for the zsh shell.

You will need to enable shell completion if is not yet enabled in your environment.
To do this, run the following command:

$ echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions for every new session, execute once:
# Linux:
$ gitlaball completion zsh > "${fpath[1]}/_gitlaball"
# macOS:
$ gitlaball completion zsh > /usr/local/share/zsh/site-functions/_gitlaball

You will need to start a new shell for this setup to take effect.
```

## Usage examples

### Create a user
Here is how you can create a new user in `main` team projects:

```
$ gitlaball users create --filter "main.*" --email=test@test.com --username=test-gitlaball --name="test gitlaball" --password="qwerty"
```

### Block a user
```
$ gitlaball users block --by=username test-gitlaball
```

Display the list of projects in which this user exists:
```
$ gitlaball users block --by=username test-gitlaball --hosts
```

### Search for a pattern in the files:
*You can search through several files at once, but that doubles the search time.*

*You can search for several patterns at once; it does not affect the search time.*

```
$ gitlaball projects files search --filepath="werf.yaml,werf.yml" --pattern="mount"
```

### Search for scheduled pipelines
The command below displays a list of projects with inactive scheduled pipelines:
```
$ gitlaball -f "main.*" projects pipelines schedules --active=false
```

### Search for a user
```
$ gitlaball users search --by=username docker
```

### List users
Display the list of users grouped by the username:
```
$ gitlaball users list --group_by=username
```

Show only those users who are active in *n* projects:
```
$ gitlaball users list --group_by=username --count n
```

Display the list of administrators:
```
$ gitlaball users list --group_by=username --admins=true
```

### Show the list of hosts specified in the config
```
$ gitlaball config list
```

### Clear the cache
```
$ gitlaball cache clean
```

### Show the list of current versions
```
$ gitlaball versions
```

### Show the active user
```
$ gitlaball whoami
```
