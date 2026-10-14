require 'json'
require 'net/http'

class InferenceClient
  def initialize(base_url: ENV.fetch('INFERENCE_URL', 'http://localhost:8080'))
    @base_url = base_url.chomp('/')
  end

  def review(diff)
    uri = URI("#{@base_url}/v1/review")
    request = Net::HTTP::Post.new(uri)
    request['Content-Type'] = 'application/json'
    request.body = JSON.generate(diff: diff)

    http = Net::HTTP.new(uri.host, uri.port)
    http.open_timeout = 5
    http.read_timeout = 60
    response = http.request(request)
    raise "Inference service request failed with status #{response.code}" unless response.is_a?(Net::HTTPSuccess)

    result = JSON.parse(response.body)
    comments = result.fetch('comments')
    raise 'Inference response comments must be an array' unless comments.is_a?(Array)

    comments
  end
end
