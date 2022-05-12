[![Go](https://github.com/flant/gitlaball/actions/workflows/go.yml/badge.svg)](https://github.com/flant/gitlaball/actions/workflows/go.yml)
[![Docker Image](https://github.com/flant/gitlaball/actions/workflows/docker.yml/badge.svg)](https://github.com/flant/gitlaball/actions/workflows/docker.yml)

# Утилита для администрирования self-hosted Gitlab инстансов

Скачать собранный бинарный файл можно на странице релизов https://github.com/flant/github.com/flant/gitlaball/releases

Сборка
```
$ git clone https://github.com/flant/gitlaball.git
$ cd gitlaball
$ go build -v -o build/gitlaball *.go
```

## Конфигурация

```
$ cat config.yaml

# Настройки кеша
cache:
  # По умолчанию кеш http-запросов включен
  enabled: true
  # По умолчанию кеш хранится в кеш директории пользователя, в зависимости от ОС
  # https://pkg.go.dev/os?utm_source=gopls#UserCacheDir
  # MacOS   : $HOME/Library/Caches/gitlaball
  # Linux   : $HOME/.cache/gitlaball
  path: ""
  # По умолчанию размер кеша 100Мб
  size: 100MB
  # По умолчанию включено GZIP сжатие кеша
  compression: true
  # По умолчанию кеш валиден в течение 1 суток
  ttl: 24h

# По умолчанию операции выполняются со всеми хостами.
# Можно использовать фильтр по команде/проекту в формате regexp (например, "main.*" или "main.example-project")
# на уровне конфига или используя флаг --filter (-f)
filter: ".*"

# Количество одновременных http подключений
threads: 100

# Список хостов (обязательный параметр)
# Итоговое имя проекта формируется как "<team>.<project>.<name>"
hosts:
  # Команда (например main)
  team:
    # Имя проекта (например, example-project)
    project:
      # Название гитлаба
      name:
        # Ссылка на гитлаб проекта
        url: https://gitlab.example.com
        # Токен юзера, от имени которого клиент будет ходить в API
        # API token - полный доступ, включая создание/удаление юзеров
        # Read API token - доступ только на чтение без возможности создавать/изменять пользователей, в том числе фильтровать список пользователей по email
        token: <api|read_api token>
        # Проверка превышения rate limit
        # При использовании кеша - выключено по умолчанию, т.к. создает дополнительные некешируемые запросы.
        # Имеет смысл включать только при включенном rate limit в гитлабе
        # https://docs.gitlab.com/ee/security/rate_limits.html
        rate_limit: false
```

### Как добавить хост
- Зайти в https://gitlab.example.com/-/profile/personal_access_tokens под своим пользователем
- Создать **Personal Access Token** с названием `gitlaball` и со скоупом:
  - `read_api` - если нужен доступ только на чтение
  - `api` - если нужен полный доступ с возможностью создания пользователей и тп (при этом у пользователя должны быть права админа)
- Добавить хост и токен в `config.yaml`, конфиг положить рядом с утилитой `gitlaball` (или указать путь к конфигу через `--config=path`)

Пример
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

Проверяем, что хост присутствует в конфиге
```
$ gitlaball config list
[main.example-project.secondary] https://gitlab-secondary.example.com
[main.example-project.primary]   https://gitlab-primary.example.com
Total: 2
```

## Использование

На данный момент реализованы следующие функции:

* Создание/блокировка/удаление/изменение пользователей (`users [create|block|delete|modify]`)
* Вывод списка пользователей, с возможностью сортировки/группировки/фильтрации (`users list`)
* Поиск определенного пользователя (`users search`)
* Вывод списка репозиториев, с возможностью сортировки/группировки/фильтрации (`projects list`)
* Поиск заданий по расписанию в репозиториях, с возможностью фильтрации по состоянию активно/неактивно (`projects pipelines schedules`)
* Поиск regex паттерна в указанных файлах в репозиториях (`projects files search`)
* Вывод списка текущих версий гитлабов с информацией о необходимости обновления (up to date|update available|update asap)
* Вывод информации о текущем пользователе API (`whoami`)

```
$ gitlaball -h
Bulk Gitlab instances administrator

Usage:
  gitlaball [flags]
  gitlaball [command]

Available Commands:
  cache       Cache management
  completion  Generate the autocompletion script for the specified shell
  config      Information about the current configuration
  help        Help about any command
  projects    Projects API
  users       Users API
  versions    Retrieve version information for GitLab instances
  whoami      Current API user

Flags:
      --config string      Path to the configuration file. (default "config.yaml") (default "config.yaml")
  -f, --filter string      Select Gitlab(s) by regexp filter (default ".*")
  -h, --help               help for gitlaball
      --log_level string   Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, off] (default "info")
      --threads int        Number of concurrent processes. (default: one process for each Gitlab instances in config file) (default 100)
      --ttl duration       Override cache TTL set in config file (default 24h0m0s)
  -u, --update             Refresh cache

Use "gitlaball [command] --help" for more information about a command.
```

### Autocompletion

#### Bash
```
$ gitlaball completion bash -h

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:
$ source <(gitlaball completion bash)

To load completions for every new session, execute once:
Linux:
  $ gitlaball completion bash > /etc/bash_completion.d/gitlaball
MacOS:
  $ gitlaball completion bash > /usr/local/etc/bash_completion.d/gitlaball

You will need to start a new shell for this setup to take effect.
```

#### Zsh

```
$ gitlaball completion zsh -h

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

$ echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions for every new session, execute once:
# Linux:
$ gitlaball completion zsh > "${fpath[1]}/_gitlaball"
# macOS:
$ gitlaball completion zsh > /usr/local/share/zsh/site-functions/_gitlaball

You will need to start a new shell for this setup to take effect.
```

## Примеры использования

### Создание пользователя
Пример создания нового пользователя в проектах команды main

```
$ gitlaball users create --filter "main.*" --email=test@test.com --username=test-gitlaball --name="test gitlaball" --password="qwerty"
```

### Блокировка пользователя
```
$ gitlaball users block --by=username test-gitlaball
```

Вывести только список проектов, в которых этот пользователь есть
```
$ gitlaball users block --by=username test-gitlaball --hosts
```

### Поиск контента в файлах
*Можно искать по нескольким файлам сразу, но это увеличивает время поиска как минимум в 2 раза.*

*Можно искать несколько паттернов, время это не увеличивает*

```
$ gitlaball projects files search --filepath="werf.yaml,werf.yml" --pattern="mount"
```

### Поиск запланированных пайплайнов
Например, найти проекты с неактивными заданиями по расписанию
```
$ gitlaball -f "main.*" projects pipelines schedules --active=false
```

### Поиск пользователя
```
$ gitlaball users search --by=username docker
```

### Список пользователей
С группировкой по имени пользователя
```
$ gitlaball users list --group_by=username
```

Только пользователи, которые присутствуют в *n* проектах
```
$ gitlaball users list --group_by=username --count n
```

Только администраторы
```
$ gitlaball users list --group_by=username --admins=true
```

### Список хостов из конфига
```
$ gitlaball config list
```

### Очистка кеша
```
$ gitlaball cache clean
```

### Список текущих версий
```
$ gitlaball versions
```

### Вывод текущего пользователя
```
$ gitlaball whoami
```
