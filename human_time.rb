#!/usr/bin/env ruby

# Read input line by line from stdin
STDIN.each_line do |line|
  line = line.chomp
  next if line.empty?

  # Find the last space - separator between fields and timestamp
  last_space = line.rindex(' ')
  unless last_space
    puts line
    next
  end

  prefix = line[0...last_space]          # part before timestamp
  ts_str = line[last_space + 1..-1]      # timestamp string in nanoseconds

  begin
    ts_ns = ts_str.to_i                  # nanoseconds since epoch (integer)
    seconds = ts_ns / 1_000_000_000      # whole seconds
    nanoseconds = ts_ns % 1_000_000_000  # remaining nanoseconds

    # Create time object in UTC (InfluxDB stores timestamps in UTC)
    time = Time.at(seconds, nanoseconds, :nsec)

    # Format: YYYY-MM-DD HH:MM:SS.nanoseconds (9 digits)
    human_time = time.utc.strftime('%Y-%m-%d %H:%M:%S.%9N')

    puts "#{prefix} #{human_time}"
  rescue => e
    # On parsing error, output the line unchanged
    puts line
  end
end