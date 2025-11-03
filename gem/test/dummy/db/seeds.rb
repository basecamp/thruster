# Load test data for Thruster gallery demo

puts "Creating Thruster gallery..."
gallery = Gallery.find_or_create_by!(name: "Thruster")

# Clear existing media
gallery.media.purge

puts "Attaching test files..."
fixtures_path = Rails.root.join("../../test/fixtures/files")

Dir.glob(fixtures_path.join("*")).each do |file_path|
  next unless File.file?(file_path)

  filename = File.basename(file_path)

  # Determine content type based on extension
  content_type = case File.extname(filename).downcase
  when ".png", ".jpg", ".jpeg", ".gif", ".webp"
    "image/#{File.extname(filename).delete('.')}"
  when ".mp4", ".mov", ".avi", ".webm"
    "video/#{File.extname(filename).delete('.')}"
  when ".pdf"
    "application/pdf"
  else
    "application/octet-stream"
  end

  gallery.media.attach(
    io: File.open(file_path),
    filename: filename,
    content_type: content_type
  )

  puts "  ✓ Attached #{filename}"
end

puts "\nGallery '#{gallery.name}' loaded with #{gallery.media.count} media files"
puts "Visit http://localhost:3000 to view the gallery!"
