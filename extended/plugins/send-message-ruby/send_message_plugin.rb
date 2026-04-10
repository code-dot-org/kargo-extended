#!/usr/bin/env ruby

require_relative "lib/send_message_plugin"

SendMessagePlugin.run! if $PROGRAM_NAME == __FILE__
