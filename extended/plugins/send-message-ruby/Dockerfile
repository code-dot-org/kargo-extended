FROM ruby:3.4.7-alpine3.22

RUN gem install --no-document webrick

RUN addgroup -S plugin && adduser -S -G plugin -u 65532 plugin

WORKDIR /app

COPY send_message_plugin.rb /app/send_message_plugin.rb
COPY lib /app/lib

USER 65532:65532

ENTRYPOINT ["ruby", "/app/send_message_plugin.rb"]
