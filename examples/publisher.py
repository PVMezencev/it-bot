import json

import pika

if __name__ == '__main__':
  # Создаём подключение.
  connection = pika.BlockingConnection(pika.ConnectionParameters(
    host='127.0.0.1',
    port=5672,
    credentials=pika.PlainCredentials('guest', 'guest')
  ))
  channel = connection.channel()

  # Пример содержимого сообщения.
  msg_payload = {
    'text': 'Привет из Раббит!',
    'recipient': 000000000,  # известный боту ID чата в телеграм.
  }
  # Приводим сообщение в массив байт JSON.
  data = json.dumps(msg_payload, ensure_ascii=False)

  # Отправляем сообщение в очередь.
  channel.basic_publish(exchange='',
                        routing_key='events',
                        body=data)
  # Закрываем соединение с сервером очередей.
  connection.close()
