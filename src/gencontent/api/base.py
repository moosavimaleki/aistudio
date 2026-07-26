"""تنظیم مشترک مدل‌های ورودی HTTP."""

from pydantic import BaseModel, ConfigDict


class ApiModel(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)
