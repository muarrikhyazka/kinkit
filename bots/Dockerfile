FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY kinkit_common.py chat_helpers.py telegram_bot.py discord_bot.py slack_bot.py matrix_bot.py ./
