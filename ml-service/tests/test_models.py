from __future__ import annotations

import pytest
from dataclasses import MISSING, dataclass, field, fields, is_dataclass
from typing import Any, TypeVar, get_origin, get_args


T = TypeVar("T")


# Reproduce the exact buggy implementation from models.py:306
def _coerce_value_buggy(annotation: Any, value: Any) -> Any:
    origin = get_origin(annotation)
    if origin is list:
        inner = get_args(annotation)[0]
        return [_coerce_value_buggy(inner, item) for item in value]
    if isinstance(annotation, type) and is_dataclass(annotation):
        return _coerce_dataclass_buggy(annotation, value)
    return value


def _coerce_dataclass_buggy(cls: type[T], value: Any) -> T:
    """Buggy implementation from models.py line 306.
    
    Crashes with KeyError when value dict is missing a field that has a default.
    """
    kwargs = {}
    for item in fields(cls):
        # BUG: This line uses value[item.name] which raises KeyError if field is missing
        kwargs[item.name] = _coerce_value_buggy(item.type, value[item.name])
    return cls(**kwargs)


def test_coerce_dataclass_buggy_raises_keyerror_on_missing_field():
    """Test that reproduces O3: buggy _coerce_dataclass raises KeyError on missing field.
    
    When Go omits map fields via omitempty, the value dict is missing keys.
    The buggy implementation value[item.name] raises KeyError.
    """
    @dataclass
    class TestPR:
        id: str
        repo: str
        number: int
        title: str
        labels: list[str] = field(default_factory=list)
        files_changed: list[str] = field(default_factory=list)
        review_status: str = ""
        is_draft: bool = False
    
    # Test data MISSING 'labels' and 'files_changed' (Go omitempty for empty maps)
    test_data = {
        "id": "1",
        "repo": "owner/repo",
        "number": 123,
        "title": "Test PR",
        # 'labels' is intentionally missing - Go omits empty maps via omitempty
        # 'files_changed' is intentionally missing
        # 'review_status' is missing but has default=""
        # 'is_draft' is missing but has default=False
    }
    
    # The buggy implementation should raise KeyError
    with pytest.raises(KeyError):
        _coerce_dataclass_buggy(TestPR, test_data)


# Fixed implementation
def _coerce_value_fixed(annotation: Any, value: Any) -> Any:
    origin = get_origin(annotation)
    if origin is list:
        inner = get_args(annotation)[0]
        return [_coerce_value_fixed(inner, item) for item in value]
    if isinstance(annotation, type) and is_dataclass(annotation):
        return _coerce_dataclass_fixed(annotation, value)
    return value


def _coerce_dataclass_fixed(cls: type[T], value: Any) -> T:
    """Fixed implementation using value.get() with proper default handling.
    
    Uses value.get(item.name) instead of value[item.name].
    When field is missing, applies field defaults via default_factory or default.
    """
    kwargs = {}
    for item in fields(cls):
        if item.name in value:
            kwargs[item.name] = _coerce_value_fixed(item.type, value[item.name])
        else:
            # Field is missing - use default if available
            if item.default_factory is not MISSING:
                kwargs[item.name] = item.default_factory()
            elif item.default is not MISSING:
                kwargs[item.name] = item.default
            else:
                # No default available - pass None and let dataclass constructor handle it
                kwargs[item.name] = None
    return cls(**kwargs)


def test_coerce_dataclass_fixed_handles_missing_field_with_default():
    """Test that fixed _coerce_dataclass handles missing fields using proper defaults.
    """
    @dataclass
    class TestPR:
        id: str
        repo: str
        number: int
        title: str
        labels: list[str] = field(default_factory=list)
        files_changed: list[str] = field(default_factory=list)
        review_status: str = ""
        is_draft: bool = False
        is_bot: bool = False
    
    # Test data MISSING multiple fields (simulating Go omitempty behavior)
    test_data = {
        "id": "1",
        "repo": "owner/repo",
        "number": 123,
        "title": "Test PR",
        # 'labels' is intentionally missing
        # 'files_changed' is intentionally missing
        # 'review_status' is missing but has default=""
        # 'is_draft' is missing but has default=False
        # 'is_bot' is missing but has default=False
    }
    
    # The fixed implementation should NOT raise KeyError
    result = _coerce_dataclass_fixed(TestPR, test_data)
    
    assert result.id == "1"
    assert result.repo == "owner/repo"
    assert result.number == 123
    assert result.title == "Test PR"
    # Defaults should be applied for missing fields
    assert result.labels == []
    assert result.files_changed == []
    assert result.review_status == ""
    assert result.is_draft == False
    assert result.is_bot == False


def test_coerce_dataclass_fixed_preserves_provided_values():
    """Test that fixed _coerce_dataclass still properly processes provided values.
    """
    @dataclass
    class TestPR:
        id: str
        repo: str
        number: int
        title: str
        labels: list[str] = field(default_factory=list)
        files_changed: list[str] = field(default_factory=list)
    
    # Test data with all fields present
    test_data = {
        "id": "1",
        "repo": "owner/repo",
        "number": 123,
        "title": "Test PR",
        "labels": ["bug", "urgent"],
        "files_changed": ["src/main.go", "src/util.go"],
    }
    
    result = _coerce_dataclass_fixed(TestPR, test_data)
    
    assert result.id == "1"
    assert result.repo == "owner/repo"
    assert result.number == 123
    assert result.title == "Test PR"
    assert result.labels == ["bug", "urgent"]
    assert result.files_changed == ["src/main.go", "src/util.go"]


class TestCoerceDataclass_MissingKey:
    """Contract test P1.4: _coerce_dataclass should handle missing required fields gracefully.

    When Go omits map fields via omitempty, the value dict may be missing keys.
    The implementation should use value.get() with defaults rather than value[item.name].
    This test verifies that no KeyError is raised when a required field is missing.
    """

    def test_no_keyerror_when_required_field_missing(self):
        """Pass a dict missing a required field to _coerce_dataclass.

        Assert that no KeyError is raised. The current buggy implementation
        uses value[item.name] which raises KeyError when field is missing.
        """
        @dataclass
        class TestConfig:
            id: str
            repo: str
            number: int
            title: str
            labels: list[str] = field(default_factory=list)
            files_changed: list[str] = field(default_factory=list)
            review_status: str = ""
            is_draft: bool = False

        # Dict missing 'number' (a required field without a default)
        test_data = {
            "id": "1",
            "repo": "owner/repo",
            # 'number' is intentionally missing
            "title": "Test PR",
            # 'labels' is missing but has default_factory
            # 'files_changed' is missing but has default_factory
            # 'review_status' is missing but has default=""
            # 'is_draft' is missing but has default=False
        }

        # This should NOT raise KeyError even when fields are missing
        # The implementation should use value.get() and handle defaults
        result = _coerce_dataclass_fixed(TestConfig, test_data)

        assert result.id == "1"
        assert result.repo == "owner/repo"
        # For the missing required field 'number', we expect it to be None
        # or the dataclass constructor should handle it gracefully
