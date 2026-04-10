require "fileutils"
require "json"
require "open3"
require "tmpdir"
require "yaml"

module SendMessageRubySmoke
  class SmokeFailure < StandardError
  end

  class Shell
    def capture(*command, chdir: nil)
      stdout, stderr, status =
        if chdir
          Open3.capture3(*command, chdir: chdir)
        else
          Open3.capture3(*command)
        end
      return stdout if status.success?

      raise SmokeFailure, "#{command.join(' ')} failed\n#{stdout}#{stderr}"
    end

    def run_command(*command, chdir: nil)
      output = capture(*command, chdir: chdir)
      print output unless output.empty?
      true
    end
  end

  class SmokeRunner
    REQUIRED_ENV = %w[
      SEND_MESSAGE_SMOKE_PROJECT
      SEND_MESSAGE_SMOKE_WAREHOUSE
      SEND_MESSAGE_SMOKE_FREIGHT_NAME
      SEND_MESSAGE_SMOKE_SLACK_API_KEY
      SEND_MESSAGE_SMOKE_CHANNEL_ID
    ].freeze

    def initialize(shell: Shell.new)
      @shell = shell
      @plugin_dir = File.expand_path("..", __dir__)
      @kargo_bin = ENV.fetch("KARGO_BIN", "kargo")
      @kargo_flags = ENV.fetch("KARGO_FLAGS", "")
      @kubectl_bin = ENV.fetch("KUBECTL_BIN", "kubectl")
      @docker_bin = ENV.fetch("DOCKER_BIN", "docker")
      @kind_bin = ENV.fetch("KIND_BIN", "kind")
      @system_resources_namespace = ENV.fetch(
        "SEND_MESSAGE_SMOKE_SYSTEM_RESOURCES_NAMESPACE",
        "kargo-system-resources"
      )
      @secret_name = ENV.fetch(
        "SEND_MESSAGE_SMOKE_SECRET_NAME",
        "send-message-slack-token"
      )
      @channel_name = ENV.fetch(
        "SEND_MESSAGE_SMOKE_CHANNEL_NAME",
        "send-message-smoke"
      )
      @configmap_name = ENV.fetch(
        "SEND_MESSAGE_SMOKE_CONFIGMAP_NAME",
        "send-message-step-plugin"
      )
      @cluster_role_name = "send-message-step-plugin-reader"
      @project = ENV["SEND_MESSAGE_SMOKE_PROJECT"]
      @warehouse = ENV["SEND_MESSAGE_SMOKE_WAREHOUSE"]
      @freight_name = ENV["SEND_MESSAGE_SMOKE_FREIGHT_NAME"]
      @slack_api_key = ENV["SEND_MESSAGE_SMOKE_SLACK_API_KEY"]
      @slack_channel_id = ENV["SEND_MESSAGE_SMOKE_CHANNEL_ID"]
      @tmp_dir = nil
      @stage_name = nil
      @promotion_name = nil
      @cluster_role_binding_name = nil
    end

    def run
      require_env!
      ensure_kind_context!
      build_and_load_image
      render_plugin_dir
      build_plugin_configmap
      install_manifests
      create_test_resources
      run_stage
      poll_promotion
      pass("send-message StepPlugin smoke promotion finished successfully")
    ensure
      cleanup
    end

    private

    def require_env!
      missing = REQUIRED_ENV.select { |name| ENV[name].to_s.empty? }
      return if missing.empty?

      noun = missing.length == 1 ? "is" : "are"
      raise SmokeFailure, "#{missing.join(', ')} #{noun} required"
    end

    def ensure_kind_context!
      context = capture(@kubectl_bin, "config", "current-context").strip
      return if context.start_with?("kind-")

      raise SmokeFailure, "send-message smoke requires a kind context, got: #{context}"
    end

    def build_and_load_image
      kind_name = capture(@kubectl_bin, "config", "current-context").strip.delete_prefix("kind-")
      @image_tag = "send-message-step-plugin-ruby:e2e-#{Time.now.to_i}"

      info("Build send-message StepPlugin image")
      run_command(@docker_bin, "build", "-t", @image_tag, @plugin_dir)
      pass("Build send-message StepPlugin image")

      info("Load send-message StepPlugin image into kind")
      run_command(@kind_bin, "load", "docker-image", "--name", kind_name, @image_tag)
      pass("Load send-message StepPlugin image into kind")
    end

    def render_plugin_dir
      @tmp_dir = Dir.mktmpdir("send-message-stepplugin-ruby-")
      plugin_yaml = File.read(File.join(@plugin_dir, "plugin.yaml"))
      plugin_yaml = plugin_yaml.gsub("namespace: kargo-system-resources", "namespace: #{@system_resources_namespace}")
      plugin_yaml = plugin_yaml.gsub("image: send-message-step-plugin-ruby:dev", "image: #{@image_tag}")
      plugin_yaml = plugin_yaml.gsub("value: kargo-system-resources", "value: #{@system_resources_namespace}")
      File.write(File.join(@tmp_dir, "plugin.yaml"), plugin_yaml)
    end

    def build_plugin_configmap
      info("Build send-message StepPlugin ConfigMap")
      run_command(@kargo_bin, "step-plugin", "build", ".", chdir: @tmp_dir)
      pass("Build send-message StepPlugin ConfigMap")
    end

    def install_manifests
      info("Install send-message CRDs")
      run_command(@kubectl_bin, "apply", "-f", File.join(@plugin_dir, "manifests", "crds.yaml"))
      pass("Install send-message CRDs")

      info("Install send-message ClusterRole")
      run_command(@kubectl_bin, "apply", "-f", File.join(@plugin_dir, "manifests", "rbac.yaml"))
      pass("Install send-message ClusterRole")

      @cluster_role_binding_name = "send-message-step-plugin-reader-#{@project}"
      binding_path = File.join(@tmp_dir, "#{@cluster_role_binding_name}.yaml")
      File.write(binding_path, YAML.dump(cluster_role_binding))
      info("Bind send-message ClusterRole to test project default ServiceAccount")
      run_command(@kubectl_bin, "apply", "-f", binding_path)
      pass("Bind send-message ClusterRole to test project default ServiceAccount")

      info("Install send-message StepPlugin ConfigMap in system resources namespace")
      run_command(
        @kubectl_bin,
        "apply",
        "-f",
        File.join(@tmp_dir, "#{@configmap_name}-configmap.yaml")
      )
      pass("Install send-message StepPlugin ConfigMap in system resources namespace")
    end

    def create_test_resources
      info("Create send-message Slack Secret")
      run_command(
        @kubectl_bin,
        "apply",
        "-f",
        write_yaml("#{@secret_name}.yaml", secret_manifest)
      )
      pass("Create send-message Slack Secret")

      info("Create send-message MessageChannel")
      run_command(
        @kubectl_bin,
        "apply",
        "-f",
        write_yaml("#{@channel_name}.yaml", channel_manifest)
      )
      pass("Create send-message MessageChannel")
    end

    def run_stage
      @stage_name = "smsgrb-#{Time.now.to_i}"
      info("Create send-message StepPlugin smoke stage")
      output = capture(
        @kargo_bin,
        "apply",
        "-f",
        write_yaml("#{@stage_name}.yaml", stage_manifest),
        *split_flags
      )
      puts output
      unless output.include?("stage.kargo.akuity.io/#{@stage_name}")
        raise SmokeFailure, "send-message smoke stage apply output did not mention #{@stage_name}"
      end
      pass("Create send-message StepPlugin smoke stage")

      info("Approve freight for send-message StepPlugin smoke stage")
      run_command(
        @kargo_bin,
        "approve",
        "--project=#{@project}",
        "--freight=#{@freight_name}",
        "--stage=#{@stage_name}",
        *split_flags
      )
      pass("Approve freight for send-message StepPlugin smoke stage")

      @promotion_name = "#{@stage_name}.manual"
      info("Create send-message StepPlugin smoke promotion")
      run_command(
        @kubectl_bin,
        "apply",
        "-f",
        write_yaml("#{@promotion_name}.yaml", promotion_manifest)
      )
      pass("Create send-message StepPlugin smoke promotion")
    end

    def poll_promotion
      90.times do
        promotion_resource = fetch_promotion
        phase = promotion_resource.dig("status", "phase").to_s
        case phase
        when "Succeeded"
          thread_ts = promotion_resource.dig("status", "stepExecutionMetadata", 0, "output", "slack", "threadTS")
          thread_ts ||= promotion_resource.dig("status", "state", "step-1", "slack", "threadTS")
          return unless thread_ts.to_s.empty?

          raise SmokeFailure, "send-message smoke did not produce slack.threadTS output"
        when "Failed", "Errored", "Aborted"
          dump = capture(
            @kubectl_bin,
            "get",
            "promotion.kargo.akuity.io",
            promotion_resource.dig("metadata", "name"),
            "-n",
            @project,
            "-o",
            "yaml"
          )
          raise SmokeFailure, "send-message smoke promotion reached terminal phase #{phase}\n#{dump}"
        end
        sleep 2
      end

      dump = capture(
        @kubectl_bin,
        "get",
        "promotion.kargo.akuity.io",
        "-n",
        @project,
        "-o",
        "yaml"
      )
      raise SmokeFailure, "send-message smoke promotion did not succeed in time\n#{dump}"
    end

    def fetch_promotion
      return {} unless @promotion_name

      payload = capture(
        @kubectl_bin,
        "get",
        "promotion.kargo.akuity.io",
        @promotion_name,
        "-n",
        @project,
        "-o",
        "json"
      )
      JSON.parse(payload)
    rescue JSON::ParserError
      {}
    rescue SmokeFailure
      {}
    end

    def cleanup
      delete("stage.kargo.akuity.io", @stage_name, namespace: @project) if @stage_name
      delete("promotion.kargo.akuity.io", @promotion_name, namespace: @project) if @promotion_name
      delete("messagechannel.ee.kargo.akuity.io", @channel_name, namespace: @project)
      delete("secret", @secret_name, namespace: @project)
      delete("configmap", @configmap_name, namespace: @system_resources_namespace)
      delete("clusterrolebinding", @cluster_role_binding_name) if @cluster_role_binding_name
      delete("clusterrole", @cluster_role_name)
      run_command(
        @kubectl_bin,
        "delete",
        "-f",
        File.join(@plugin_dir, "manifests", "crds.yaml"),
        "--ignore-not-found"
      )
    rescue StandardError
      nil
    ensure
      FileUtils.remove_entry(@tmp_dir) if @tmp_dir && File.exist?(@tmp_dir)
    end

    def delete(kind, name, namespace: nil)
      return if name.to_s.empty?

      command = [@kubectl_bin, "delete", kind, name]
      command += ["-n", namespace] if namespace
      command << "--ignore-not-found"
      run_command(*command)
    end

    def cluster_role_binding
      {
        "apiVersion" => "rbac.authorization.k8s.io/v1",
        "kind" => "ClusterRoleBinding",
        "metadata" => {
          "name" => @cluster_role_binding_name
        },
        "subjects" => [
          {
            "kind" => "ServiceAccount",
            "name" => "default",
            "namespace" => @project
          }
        ],
        "roleRef" => {
          "apiGroup" => "rbac.authorization.k8s.io",
          "kind" => "ClusterRole",
          "name" => @cluster_role_name
        }
      }
    end

    def secret_manifest
      {
        "apiVersion" => "v1",
        "kind" => "Secret",
        "metadata" => {
          "name" => @secret_name,
          "namespace" => @project
        },
        "type" => "Opaque",
        "stringData" => {
          "apiKey" => @slack_api_key
        }
      }
    end

    def channel_manifest
      {
        "apiVersion" => "ee.kargo.akuity.io/v1alpha1",
        "kind" => "MessageChannel",
        "metadata" => {
          "name" => @channel_name,
          "namespace" => @project
        },
        "spec" => {
          "secretRef" => {
            "name" => @secret_name
          },
          "slack" => {
            "channelID" => @slack_channel_id
          }
        }
      }
    end

    def stage_manifest
      {
        "apiVersion" => "kargo.akuity.io/v1alpha1",
        "kind" => "Stage",
        "metadata" => {
          "name" => @stage_name,
          "namespace" => @project
        },
        "spec" => {
          "requestedFreight" => [
            {
              "origin" => {
                "kind" => "Warehouse",
                "name" => @warehouse
              },
              "sources" => {
                "direct" => true
              }
            }
          ],
          "promotionTemplate" => {
            "spec" => {
              "steps" => [
                {
                  "uses" => "send-message",
                  "config" => {
                    "channel" => {
                      "kind" => "MessageChannel",
                      "name" => @channel_name
                    },
                    "message" => "send-message StepPlugin smoke from the Ruby implementation #{@stage_name}"
                  }
                }
              ]
            }
          }
        }
      }
    end

    def promotion_manifest
      {
        "apiVersion" => "kargo.akuity.io/v1alpha1",
        "kind" => "Promotion",
        "metadata" => {
          "name" => @promotion_name,
          "namespace" => @project
        },
        "spec" => {
          "stage" => @stage_name,
          "freight" => @freight_name,
          "steps" => stage_manifest.dig(
            "spec",
            "promotionTemplate",
            "spec",
            "steps"
          )
        }
      }
    end

    def split_flags
      @kargo_flags.split.reject(&:empty?)
    end

    def write_yaml(name, object)
      path = File.join(@tmp_dir, name)
      File.write(path, YAML.dump(object))
      path
    end

    def capture(*command, chdir: nil)
      @shell.capture(*command, chdir: chdir)
    end

    def run_command(*command, chdir: nil)
      @shell.run_command(*command, chdir: chdir)
    end

    def info(message)
      puts "[INFO] #{message}"
    end

    def pass(message)
      puts "[PASS] #{message}"
    end
  end
end
