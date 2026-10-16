FROM python:3.12.11-slim
COPY deploy/analysis-sandbox/requirements.txt /opt/judex/requirements.txt
RUN pip install --no-cache-dir -r /opt/judex/requirements.txt
COPY deploy/analysis-sandbox/analyze_material.py /opt/judex/analyze_material.py
RUN mkdir -p /workspace && chmod 755 /opt/judex/analyze_material.py
WORKDIR /workspace
CMD ["sleep", "infinity"]
