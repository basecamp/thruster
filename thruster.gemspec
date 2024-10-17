require_relative "lib/thruster/version"

Gem::Specification.new do |s|
  s.name        = "thruster"
  s.version     = Thruster::VERSION
  s.summary     = "Zero-config HTTP/2 proxy"
  s.description = "A zero-config HTTP/2 proxy for lightweight production deployments"
  s.authors     = [ "Kevin McConnell" ]
  s.email       = "kevin@37signals.com"
  s.homepage    = "https://github.com/basecamp/thruster"
  s.license     = "MIT"

  s.metadata = {
    "homepage_uri" => s.homepage,
    "rubygems_mfa_required" => "true"
  }

  s.files = Dir[ "{lib}/**/*", "MIT-LICENSE", "README.md" ]
  s.bindir = "exe"
  s.executables << "thrust"

  s.add_dependency "railties", ">= 7.2"
end
