#!/usr/bin/env ruby

# Читаем построчно из стандартного ввода
STDIN.each_line do |line|
  line = line.chomp
  next if line.empty?

  # Ищем последний пробел — разделитель между набором полей и временной меткой
  last_space = line.rindex(' ')
  unless last_space
    puts line
    next
  end

  prefix = line[0...last_space]          # часть до timestamp
  ts_str = line[last_space + 1..-1]      # строка с наносекундами

  begin
    ts_ns = ts_str.to_i                  # наносекунды с эпохи (целое)
    seconds = ts_ns / 1_000_000_000      # целые секунды
    nanoseconds = ts_ns % 1_000_000_000  # остаток в наносекундах

    # Создаём объект времени в UTC (InfluxDB хранит время в UTC)
    time = Time.at(seconds, nanoseconds, :nsec)

    # Форматируем: ГГГГ-ММ-ДД ЧЧ:ММ:СС.наносекунды (9 цифр)
    human_time = time.utc.strftime('%Y-%m-%d %H:%M:%S.%9N')

    puts "#{prefix} #{human_time}"
  rescue => e
    # В случае ошибки парсинга выводим строку без изменений
    puts line
  end
end