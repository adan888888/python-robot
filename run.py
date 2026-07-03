from __future__ import annotations

import asyncio
import logging
import os
import sys

import uvicorn

from app.config import load_config, resolve_task_file_path, task_file_path
from app.main import create_app
from app.monitor.account_listener import run_monitor

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
)
# 屏蔽 Telethon 同步频道更新时的 INFO 日志（如 "Got difference for channel ... updates"）
for _name in ("telethon", "telethon.client", "telethon.client.updates"):
    logging.getLogger(_name).setLevel(logging.WARNING)
logger = logging.getLogger(__name__)


async def _serve(conf) -> None:
    app = create_app(conf)
    config = uvicorn.Config(app, host="0.0.0.0", port=5000, log_level="warning")
    server = uvicorn.Server(config)
    await server.serve()


async def _async_main(config_path: str) -> None:
    conf = load_config(config_path)

    if not conf.monitor_enabled:
        raise SystemExit("请在 config.yaml 中开启 monitor.enabled")

    task_file = resolve_task_file_path(task_file_path(conf))
    logger.info("个人账号监听已开启，以下群的消息将写入：%s", task_file)
    for group in conf.monitor_groups:
        logger.info("  - %s", group.name or group.chat_id)

    monitor_task = asyncio.create_task(run_monitor(conf))
    http_task = asyncio.create_task(_serve(conf))

    logger.info("服务已启动")
    done, pending = await asyncio.wait(
        {monitor_task, http_task},
        return_when=asyncio.FIRST_COMPLETED,
    )
    for task in pending:
        task.cancel()
    for task in done:
        exc = task.exception()
        if exc:
            raise exc


def main() -> None:
    config_path = os.getenv("CONFIG_PATH", "config.yaml")
    if len(sys.argv) > 1:
        config_path = sys.argv[1]

    try:
        asyncio.run(_async_main(config_path))
    except KeyboardInterrupt:
        logger.info("收到退出信号，服务停止")


if __name__ == "__main__":
    main()
