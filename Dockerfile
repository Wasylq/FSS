FROM python:3.11-slim

RUN  mkdir -p /app
WORKDIR /app
COPY . /app

RUN pip3 install -r requirements.txt
RUN chmod u+x /app/sparse.py

ENTRYPOINT ["python3", "sparse.py"]
