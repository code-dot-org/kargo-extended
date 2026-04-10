module SendMessagePlugin
  class PayloadBuilder
    def initialize(config:, channel:)
      @config = config
      @channel = channel
    end

    def build
      encoding_type = @config["encodingType"].to_s.strip
      return build_plaintext if encoding_type.empty?

      payload = decode_encoded_payload(encoding_type)
      raise RequestError, "Slack payload must decode to an object" unless payload.is_a?(Hash)

      if payload["channel"].nil?
        raise RequestError, missing_encoded_channel_id_message if @channel["slack_channel_id"].to_s.empty?

        payload["channel"] = @channel["slack_channel_id"]
      end
      [payload, payload["thread_ts"].to_s]
    end

    private

    def build_plaintext
      slack_options = optional_hash(@config["slack"], "step.config.slack")
      channel_id = slack_options["channelID"].to_s.strip
      channel_id = @channel["slack_channel_id"] if channel_id.empty?
      raise RequestError, missing_plaintext_channel_id_message if channel_id.to_s.empty?

      payload = {
        "channel" => channel_id,
        "text" => @config["message"]
      }
      thread_ts = slack_options["threadTS"].to_s.strip
      payload["thread_ts"] = thread_ts unless thread_ts.empty?
      [payload, thread_ts]
    end

    def decode_encoded_payload(encoding_type)
      case encoding_type
      when "json"
        JSON.parse(@config["message"])
      when "yaml"
        Psych.safe_load(@config["message"], aliases: false)
      when "xml"
        decode_xml_payload(@config["message"])
      else
        raise RequestError, "unsupported encodingType #{encoding_type.inspect}"
      end
    rescue JSON::ParserError, Psych::SyntaxError, REXML::ParseException => error
      raise RequestError, "error decoding #{encoding_type.upcase} Slack payload: #{error.message}"
    end

    def decode_xml_payload(message)
      document = REXML::Document.new(message)
      root = document.root
      raise RequestError, "error decoding XML Slack payload: empty document" if root.nil?

      xml_root_to_hash(root)
    end

    def xml_root_to_hash(element)
      payload = {}
      element.attributes.each_attribute do |attribute|
        payload[attribute.expanded_name] = attribute.value
      end
      element.elements.each do |child|
        append_xml_value(payload, child.name, xml_element_value(child))
      end

      text = direct_text(element)
      return payload if text.empty?

      if payload.empty?
        payload["text"] = text
      else
        payload["#text"] = text
      end
      payload
    end

    def xml_element_value(element)
      return direct_text(element) if element.attributes.empty? && element.elements.empty?

      payload = {}
      element.attributes.each_attribute do |attribute|
        payload[attribute.expanded_name] = attribute.value
      end
      element.elements.each do |child|
        append_xml_value(payload, child.name, xml_element_value(child))
      end
      text = direct_text(element)
      payload["#text"] = text unless text.empty?
      payload
    end

    def direct_text(element)
      element.children.grep(REXML::Text).map(&:value).join.strip
    end

    def append_xml_value(payload, key, value)
      return payload[key] = value unless payload.key?(key)

      existing = payload[key]
      payload[key] = existing.is_a?(Array) ? existing + [value] : [existing, value]
    end

    def optional_hash(value, path)
      return {} if value.nil?
      return value if value.is_a?(Hash)

      raise RequestError, "#{path} must be an object"
    end

    def missing_plaintext_channel_id_message
      %(#{@channel["resource_kind"]} "#{@channel["resource_name"]}" does not define spec.slack.channelID and config.slack.channelID is empty)
    end

    def missing_encoded_channel_id_message
      %(#{@channel["resource_kind"]} "#{@channel["resource_name"]}" does not define spec.slack.channelID)
    end
  end
end
