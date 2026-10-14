require 'cgi'
require 'json'
require 'net/http'

class GitlabClient
  def initialize(base_url: ENV.fetch('GITLAB_API_URL', 'https://gitlab.com/api/v4'),
                 token: ENV.fetch('GITLAB_API_TOKEN', nil))
    @base_url = base_url.chomp('/')
    @token = token
  end

  def merge_request_diff(project_id, merge_request_iid)
    response = request(:get, "/projects/#{escape(project_id)}/merge_requests/#{merge_request_iid}/diffs")
    diffs = JSON.parse(response.body)
    raise 'GitLab diff response must be an array' unless diffs.is_a?(Array)

    diffs.map do |file|
      old_path = file.fetch('old_path')
      new_path = file.fetch('new_path')
      body = file.fetch('diff')
      "diff --git a/#{old_path} b/#{new_path}\n--- a/#{old_path}\n+++ b/#{new_path}\n#{body}"
    end.join("\n")
  end

  def post_review_notes(review_run, comments)
    project_id = review_run.repository.external_id
    comments.each do |comment|
      note = "#{comment.fetch('severity').upcase} (#{comment.fetch('category')}) " \
             "`#{comment.fetch('file')}:#{comment.fetch('line')}`: #{comment.fetch('message')}"
      request(
        :post,
        "/projects/#{escape(project_id)}/merge_requests/#{review_run.merge_request_iid}/notes",
        { body: note }
      )
    end
  end

  private

  def escape(value)
    CGI.escape(value.to_s)
  end

  def request(method, path, payload = nil)
    raise 'GITLAB_API_TOKEN is required' if @token.blank?

    uri = URI("#{@base_url}#{path}")
    http = Net::HTTP.new(uri.host, uri.port)
    http.use_ssl = uri.scheme == 'https'
    http.open_timeout = 5
    http.read_timeout = 30
    request = method == :post ? Net::HTTP::Post.new(uri) : Net::HTTP::Get.new(uri)
    request['PRIVATE-TOKEN'] = @token
    request['Accept'] = 'application/json'
    request['Content-Type'] = 'application/json' if payload
    request.body = JSON.generate(payload) if payload
    response = http.request(request)
    raise "GitLab API request failed with status #{response.code}" unless response.is_a?(Net::HTTPSuccess)

    response
  end
end
