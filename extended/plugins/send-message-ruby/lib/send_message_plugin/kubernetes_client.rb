module SendMessagePlugin
  class KubernetesClient
    DEFAULT_KUBERNETES_HOST = "kubernetes.default.svc"
    DEFAULT_KUBERNETES_PORT = "443"
    DEFAULT_SERVICE_ACCOUNT_DIR = "/var/run/secrets/kubernetes.io/serviceaccount"

    def initialize(
      host: ENV.fetch("KUBERNETES_SERVICE_HOST", DEFAULT_KUBERNETES_HOST),
      port: ENV.fetch("KUBERNETES_SERVICE_PORT_HTTPS", DEFAULT_KUBERNETES_PORT),
      token_path: File.join(DEFAULT_SERVICE_ACCOUNT_DIR, "token"),
      ca_path: File.join(DEFAULT_SERVICE_ACCOUNT_DIR, "ca.crt")
    )
      @base_uri = URI("https://#{host}:#{port}")
      @token_path = token_path
      @ca_path = ca_path
    end

    def get_message_channel(namespace, name)
      payload = get_json("/apis/ee.kargo.akuity.io/v1alpha1/namespaces/#{namespace}/messagechannels/#{name}")
      build_channel(payload)
    rescue RequestError
      raise
    rescue StandardError => error
      raise ExecutionError, "error getting MessageChannel: #{error.message}"
    end

    def get_cluster_message_channel(name)
      payload = get_json("/apis/ee.kargo.akuity.io/v1alpha1/clustermessagechannels/#{name}")
      build_channel(payload)
    rescue RequestError
      raise
    rescue StandardError => error
      raise ExecutionError, "error getting ClusterMessageChannel: #{error.message}"
    end

    def get_secret(namespace, name)
      payload = get_json("/api/v1/namespaces/#{namespace}/secrets/#{name}")
      decode_secret(payload)
    rescue StandardError => error
      raise ExecutionError, "error getting Slack Secret: #{error.message}"
    end

    private

    def get_json(path)
      request = Net::HTTP::Get.new(path)
      request[AUTH_HEADER] = "#{BEARER_PREFIX}#{File.read(@token_path).strip}"

      response = http_client.start do |http|
        http.request(request)
      end

      parsed = response.body.to_s.empty? ? {} : JSON.parse(response.body)
      return parsed if response.code.to_i < 400

      detail = parsed["message"].to_s
      detail = response.message if detail.empty?
      raise ExecutionError, detail.empty? ? "HTTP #{response.code}" : detail
    rescue Errno::ENOENT => error
      raise ExecutionError, error.message
    end

    def http_client
      @http_client ||= Net::HTTP.new(@base_uri.host, @base_uri.port).tap do |http|
        http.use_ssl = true
        http.ca_file = @ca_path if File.exist?(@ca_path)
        http.verify_mode = OpenSSL::SSL::VERIFY_PEER
      end
    end

    def build_channel(payload)
      {
        "secret_name" => payload.dig("spec", "secretRef", "name").to_s.strip,
        "slack_channel_id" => payload.dig("spec", "slack", "channelID").to_s.strip,
        "resource_kind" => payload["kind"].to_s,
        "resource_name" => payload.dig("metadata", "name").to_s
      }.tap do |channel|
        if channel["secret_name"].empty?
          raise RequestError, %(#{channel["resource_kind"]} "#{channel["resource_name"]}" does not define spec.secretRef.name)
        end
      end
    end

    def decode_secret(payload)
      data = payload["data"]
      return {} unless data.is_a?(Hash)

      data.each_with_object({}) do |(key, value), decoded|
        decoded[key] = value.nil? ? "" : value.unpack1("m0")
      end
    end
  end
end
