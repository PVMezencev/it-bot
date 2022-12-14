### Бот для телеграм, который берёт сообщения из очереди "events" в брокере RabbitMQ и отправляет их указанным получателям.

#### Требования:
1. Go ~> 1.8
2. Ubuntu/Debian
3. RabbitMQ 3.10.7

#### Сборка:
```bash
git clone git@github.com:PVMezencev/it-bot.git

cd it-bot

make build
```

#### Запуск:
```bash
cp configs/config.example.yml configs/config.yml
```

- заполните файл конфигурации.

```bash
./it-bot
```

#### Дополнительно:
1. configs/docker-compose.example.yml можно использовать для запуска RabbitMQ в docker.
2. examples/publisher.py - пример отправки в очередь сообщения на Python.

