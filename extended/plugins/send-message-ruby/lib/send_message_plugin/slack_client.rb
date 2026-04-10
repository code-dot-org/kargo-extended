module SendMessagePlugin
  class SlackClient
    def post_message(api_base_url:, token:, payload:)
      uri = URI("#{api_base_url.sub(%r{/\z}, "")}/chat.postMessage")
      request = Net::HTTP::Post.new(uri)
      request[AUTH_HEADER] = "#{BEARER_PREFIX}#{token}"
      request["Content-Type"] = "application/json"
      request.body = JSON.generate(payload)

      response = Net::HTTP.start(uri.host, uri.port, use_ssl: uri.scheme == "https") do |http|
        http.request(request)
      end

      parsed = response.body.to_s.empty? ? {} : JSON.parse(response.body)
      if response.code.to_i >= 400
        parsed["ok"] = false
        parsed["error"] = response.message if parsed["error"].to_s.empty?
      end
      parsed
    end
  end
end
