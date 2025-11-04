require "test_helper"

class ThrusterTest < ActiveSupport::TestCase
  test "it has a version number" do
    assert Thruster::VERSION
  end

  test "active_storage_integration_enabled?" do
    original_secret = ENV["THRUSTER_SECRET"]

    ENV["THRUSTER_SECRET"] = "test_secret"
    assert Thruster.active_storage_integration_enabled?

    ENV["THRUSTER_SECRET"] = nil
    assert_not Thruster.active_storage_integration_enabled?
  ensure
    ENV["THRUSTER_SECRET"] = original_secret
  end
end
