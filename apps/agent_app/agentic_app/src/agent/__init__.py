"""Agent layer — LangGraph orchestration.

Nodes depend only on ``domain`` ports (injected via the node factories)
plus LangGraph itself. No DB drivers, LLM SDKs, or adapter imports.
"""
