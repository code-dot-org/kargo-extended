module SendMessagePlugin
  class App
    def initialize(
      token_path: DEFAULT_TOKEN_PATH,
      system_resources_namespace: ENV.fetch(
        "SYSTEM_RESOURCES_NAMESPACE",
        DEFAULT_SYSTEM_RESOURCES_NAMESPACE
      ),
      slack_api_base_url: ENV.fetch(
        "SLACK_API_BASE_URL",
        DEFAULT_SLACK_API_BASE_URL
      ),
      kubernetes_client: nil,
      slack_client: nil,
      logger: nil
    )
      @token_path = token_path
      @system_resources_namespace = system_resources_namespace
      @slack_api_base_url = slack_api_base_url
      @kubernetes_client = kubernetes_client || KubernetesClient.new
      @slack_client = slack_client || SlackClient.new
      @logger = logger || Logger.new($stderr, level: Logger::WARN)
    end

    def call(method:, path:, headers:, body:)
      return [404, nil] if path != STEP_EXECUTE_PATH
      return [405, nil] if method != "POST"

      authorize!(headers[AUTH_HEADER] || headers[AUTH_HEADER.downcase])
      request = JSON.parse(body)
      [200, execute(request)]
    rescue AuthError => error
      [403, errored(error.message, error.message)]
    rescue JSON::ParserError => error
      [400, errored("invalid request body", error.message)]
    rescue RequestError => error
      [200, errored(error.message, error.message)]
    rescue ExecutionError => error
      [200, failed(error.message)]
    rescue StandardError => error
      [200, failed(error.message)]
    end

    def serve(req, res)
      headers = req.header.each_with_object({}) do |(key, value), memo|
        memo[key] = Array(value).first
      end
      status, response = call(
        method: req.request_method,
        path: req.path,
        headers: headers,
        body: req.body.to_s
      )
      if response.nil?
        res.status = status
        return
      end

      res.status = status
      res["Content-Type"] = "application/json"
      res.body = JSON.generate(response)
    rescue StandardError => error
      @logger.error(error.full_message)
      res.status = 500
      res["Content-Type"] = "application/json"
      res.body = JSON.generate(failed(error.message))
    end

    private

    def authorize!(header_value)
      expected = File.read(@token_path).strip
      raise AuthError, "missing bearer token" unless header_value&.start_with?(BEARER_PREFIX)

      received = header_value.delete_prefix(BEARER_PREFIX).strip
      raise AuthError, "invalid bearer token" if received != expected
    rescue Errno::ENOENT => error
      raise AuthError, "error reading auth token: #{error.message}"
    end

    def execute(request)
      step = fetch_hash(request, "step", "step")
      config = fetch_hash(step, "config", "step.config")
      context = optional_hash(request["context"], "context")
      raise RequestError, "unsupported step kind #{step["kind"].inspect}" unless step["kind"].to_s == "send-message"
      raise RequestError, "step.config.message is required" if config["message"].nil?

      channel_ref = fetch_hash(config, "channel", "step.config.channel")
      channel_kind = channel_ref["kind"].to_s.strip
      channel_name = channel_ref["name"].to_s.strip
      raise RequestError, "step.config.channel.kind is required" if channel_kind.empty?
      raise RequestError, "step.config.channel.name is required" if channel_name.empty?

      channel, secret_namespace = lookup_channel(
        project: context["project"].to_s.strip,
        channel_kind: channel_kind,
        channel_name: channel_name
      )
      secret = @kubernetes_client.get_secret(secret_namespace, channel.fetch("secret_name"))
      token = secret["apiKey"].to_s
      raise RequestError, 'Slack Secret is missing key "apiKey"' if token.empty?

      payload, output_thread_ts = PayloadBuilder.new(
        config: config,
        channel: channel
      ).build

      slack_response = @slack_client.post_message(
        api_base_url: @slack_api_base_url,
        token: token,
        payload: payload
      )
      unless slack_response["ok"]
        detail = slack_response["error"].to_s
        detail = "unknown_error" if detail.empty?
        raise ExecutionError, "Slack API error: #{detail}"
      end

      output_thread_ts = slack_response["ts"].to_s if output_thread_ts.to_s.empty?
      {
        "status" => "Succeeded",
        "output" => {
          "slack" => {
            "threadTS" => output_thread_ts
          }
        }
      }
    end

    def lookup_channel(project:, channel_kind:, channel_name:)
      case channel_kind
      when "MessageChannel"
        raise RequestError, "step context project is required for MessageChannel" if project.empty?

        [@kubernetes_client.get_message_channel(project, channel_name), project]
      when "ClusterMessageChannel"
        [@kubernetes_client.get_cluster_message_channel(channel_name), @system_resources_namespace]
      else
        raise RequestError, "unsupported channel kind #{channel_kind.inspect}"
      end
    end

    def fetch_hash(object, key, path)
      value = object[key]
      optional_hash(value, path).tap do |hash|
        raise RequestError, "#{path} is required" if hash.empty? && !value.is_a?(Hash)
      end
    end

    def optional_hash(value, path)
      return {} if value.nil?
      return value if value.is_a?(Hash)

      raise RequestError, "#{path} must be an object"
    end

    def errored(message, error)
      {
        "status" => "Errored",
        "message" => message,
        "error" => error,
        "terminal" => true
      }
    end

    def failed(message)
      {
        "status" => "Failed",
        "message" => message,
        "error" => message,
        "terminal" => true
      }
    end
  end
end
