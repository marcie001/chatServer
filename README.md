# Chat Server Sample
I made this for my study.

## Usage
```bash
    -h="": Listen IP
    -l="/home/marcie/tcpserver.log": Log file path.
    -p=8800: Listen port
```

## How to connect
    $ telnet localhost 8800

## Commands

If input starts with ".", its command.
If username or message contains spaces, double quote them.

    .quit
Disconnect server.

    .kick username [username]...
Kick out user from server.

    .dm username [username] message
Send direct message to user. message must be double quoted.


# Chat Server Sample （日本語）
学習用に作ったチャットサーバです。

## 使い方
```bash
    -h="": Listen IP
    -l="/home/marcie/tcpserver.log": Log file path.
    -p=8800: Listen port
```

## サーバへの接続
    $ telnet localhost 8800

## コマンド

"." で入力を始めた場合、それはコマンドになります。
ユーザ名やメッセージにスペースが含まれている場合はダブルクォートで括ってください。

    .quit
サーバから切断します。

    .kick username [username]...
指定したユーザをサーバから追い出します。

    .dm username [username]... message
指定したユーザにダイレクトメッセージを送ります。メッセージはダブルクォートで括らないとならないかもしれません。
