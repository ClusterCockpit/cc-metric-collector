#!/usr/bin/env ruby
# frozen_string_literal: true

# Test verifies correct operation of raw_aggregates
# Usage: cat metrics.txt | grep mem_free | ruby test_aggregates.rb

# Parse metric line in InfluxDB Line Protocol format
# Format: metric_name,tag1=val1,tag2=val2 field1=val1 timestamp
def parse_metric(line)
  line = line.strip
  return nil if line.empty?

  parts = line.split(' ')
  return nil unless parts.length >= 2

  measurement_tags = parts[0]
  fields = parts[1]
  timestamp = parts[2]&.to_i

  # Split measurement and tags
  mt_parts = measurement_tags.split(',')
  measurement = mt_parts[0]
  tags = {}
  mt_parts[1..].each do |tag|
    key, value = tag.split('=')
    tags[key] = value if key && value
  end

  # Parse fields
  field_values = {}
  fields.split(',').each do |f|
    key, value = f.split('=')
    field_values[key] = value.to_f if key && value
  end

  {
    measurement: measurement,
    tags: tags,
    fields: field_values,
    timestamp: timestamp
  }
end

# Compute aggregates for a set of metrics
def compute_aggregates(metrics)
  values = metrics.map { |m| m[:fields]['value'] }.compact
  return nil if values.empty?

  {
    min: values.min,
    max: values.max,
    avg: values.sum / values.size,
    count: values.size
  }
end

# Main test logic
def run_test
  lines = STDIN.readlines

  # Separate raw metrics and aggregates
  raw_metrics = []
  aggregate_metrics = []

  lines.each do |line|
    parsed = parse_metric(line)
    next unless parsed

    if parsed[:measurement] == 'mem_free'
      raw_metrics << parsed
    elsif parsed[:measurement].start_with?('mem_free_')
      aggregate_metrics << parsed
    end
  end

  puts "=" * 70
  puts "mem_free metrics aggregation test"
  puts "=" * 70
  puts
  puts "Raw metrics: #{raw_metrics.size}"
  puts "Aggregated metrics: #{aggregate_metrics.size}"
  puts

  if raw_metrics.empty?
    puts "ERROR: No raw mem_free metrics found"
    exit 1
  end

  if aggregate_metrics.empty?
    puts "ERROR: No aggregated mem_free_* metrics found"
    exit 1
  end

  # Sort raw metrics by timestamp
  raw_metrics.sort_by! { |m| m[:timestamp] }

  # Group aggregates by timestamp (min/max/avg have same timestamp)
  aggregates_by_time = {}
  aggregate_metrics.each do |agg|
    ts = agg[:timestamp]
    aggregates_by_time[ts] ||= {}
    stat = agg[:tags]['stat']
    aggregates_by_time[ts][stat] = agg if stat
  end

  # Sort aggregate timestamps
  sorted_agg_times = aggregates_by_time.keys.sort

  puts "Found #{sorted_agg_times.size} aggregate sets"
  puts

  # For each aggregate set, find metrics that belong to it
  # Metrics belong to aggregate if their timestamp < aggregate timestamp
  # and >= previous aggregate timestamp (if exists)

  errors = []
  warnings = []
  checked = 0

  # Aggregate timestamp = endTime = time of last metric in interval
  # So for aggregate at time T, metrics belong to interval [prev_T, T)
  # where prev_T is the previous aggregate timestamp (or 0 for first)
  sorted_agg_times.each_with_index do |agg_time, idx|
    aggs = aggregates_by_time[agg_time]

    # Get previous aggregate time (or 0 for first)
    prev_agg_time = idx > 0 ? sorted_agg_times[idx - 1] : 0

    # Find metrics in this interval: [prev_agg_time, agg_time)
    interval_metrics = raw_metrics.select do |m|
      m[:timestamp] >= prev_agg_time && m[:timestamp] < agg_time
    end

    expected = compute_aggregates(interval_metrics)

    unless expected
      warnings << "Aggregate #{idx + 1} (#{Time.at(agg_time / 1_000_000_000.0).strftime('%H:%M:%S.%L')}): no metrics in interval"
      next
    end

    # Get actual values from aggregates
    actual_min = aggs['min']&.dig(:fields, 'value')
    actual_max = aggs['max']&.dig(:fields, 'value')
    actual_avg = aggs['avg']&.dig(:fields, 'value')

    # Check values with 0.1% tolerance for float comparison
    tolerance = 0.001

    check_value = lambda do |name, expected_val, actual_val|
      if actual_val.nil?
        errors << "Aggregate #{idx + 1}: missing #{name}"
        return
      end

      diff = (expected_val - actual_val).abs
      relative_diff = expected_val.abs > 0 ? diff / expected_val.abs : diff

      if relative_diff > tolerance
        errors << "Aggregate #{idx + 1}: #{name} expected #{expected_val.round(2)}, got #{actual_val.round(2)} (diff #{(relative_diff * 100).round(4)}%)"
      else
        checked += 1
      end
    end

    check_value.call('min', expected[:min], actual_min)
    check_value.call('max', expected[:max], actual_max)
    check_value.call('avg', expected[:avg], actual_avg)

    # Output info
    status = errors.empty? ? "OK" : "FAIL"
    time_str = Time.at(agg_time / 1_000_000_000.0).strftime('%H:%M:%S.%L')
    puts "  Aggregate #{idx + 1} [#{time_str}]: #{status} (#{expected[:count]} metrics)"
    puts "    Expected: min=#{expected[:min].round(2)}, max=#{expected[:max].round(2)}, avg=#{expected[:avg].round(2)}"
    puts "    Actual:   min=#{actual_min&.round(2)}, max=#{actual_max&.round(2)}, avg=#{actual_avg&.round(2)}"

    prev_agg_time = agg_time
  end

  puts
  puts "=" * 70

  unless warnings.empty?
    puts "Warnings:"
    warnings.each { |w| puts "  - #{w}" }
    puts
  end

  if errors.empty?
    puts "TEST PASSED! Verified #{checked} values."
    exit 0
  else
    puts "TEST FAILED!"
    puts "Errors:"
    errors.each { |e| puts "  - #{e}" }
    exit 1
  end
end

run_test