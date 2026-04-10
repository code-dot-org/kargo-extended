module SendMessagePlugin
  class RequestError < StandardError
  end

  class AuthError < RequestError
  end

  class ExecutionError < StandardError
  end
end
