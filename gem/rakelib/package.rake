require "rubygems/package_task"

NATIVE_PLATFORMS = {
  "arm64-darwin" => "../dist/thrust-darwin-arm64",
  "x86_64-darwin" => "../dist/thrust-darwin-amd64",
  "x86_64-linux" => "../dist/thrust-linux-amd64",
  "aarch64-linux" => "../dist/thrust-linux-arm64"
}

BASE_GEMSPEC = Bundler.load_gemspec("thruster.gemspec")

gem_path = Gem::PackageTask.new(BASE_GEMSPEC).define
desc "Build the ruby gem"
task "gem:ruby" => [ gem_path ]

desc "Build native executables"
namespace :build do
  task :native do
    system("make dist")
  end
end
task gem: "build:native"

NATIVE_PLATFORMS.each do |platform, executable|
  BASE_GEMSPEC.dup.tap do |gemspec|
    exedir = File.join(gemspec.bindir, platform)
    exepath = File.join(exedir, "thrust")

    gemspec.platform = platform
    gemspec.files << exepath

    gem_path = Gem::PackageTask.new(gemspec).define
    desc "Build the #{platform} gem"
    task "gem:#{platform}" => [ gem_path ]

    directory exedir
    file exepath => [ exedir ] do
      FileUtils.cp executable, exepath
      FileUtils.chmod(0755, exepath)
    end

    CLOBBER.add(exedir)
  end
end

CLOBBER.add("dist")
