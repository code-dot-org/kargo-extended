require "json"
require "logger"
require "net/http"
require "openssl"
require "psych"
require "rexml/document"
require "uri"
require "webrick"

require_relative "send_message_plugin/errors"
require_relative "send_message_plugin/kubernetes_client"
require_relative "send_message_plugin/payload_builder"
require_relative "send_message_plugin/slack_client"
require_relative "send_message_plugin/app"

module SendMessagePlugin
  STEP_EXECUTE_PATH = "/api/v1/step.execute"
  AUTH_HEADER = "Authorization"
  BEARER_PREFIX = "Bearer "
  DEFAULT_TOKEN_PATH = "/var/run/kargo/token"
  DEFAULT_SYSTEM_RESOURCES_NAMESPACE = "kargo-system-resources"
  DEFAULT_SLACK_API_BASE_URL = "https://slack.com/api"

  def self.run!
    app = App.new
    server = WEBrick::HTTPServer.new(
      BindAddress: "0.0.0.0",
      Port: Integer(ENV.fetch("PORT", "9765")),
      AccessLog: [],
      Logger: WEBrick::Log.new($stderr, WEBrick::Log::WARN)
    )
    server.mount_proc(STEP_EXECUTE_PATH) do |req, res|
      app.serve(req, res)
    end
    trap("INT") { server.shutdown }
    trap("TERM") { server.shutdown }
    server.start
  end
end
