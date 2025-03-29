from pathlib import Path

from core.base_test import GenericTestImpl
from core.environment import Environment
import core.keys as keys
import core.test_constants as tconst

TESTROOT = Path("cmd_ops")
DIR1 = TESTROOT / "dir1"
FILE1 = TESTROOT / "file1"

class CommandTest(GenericTestImpl):
    """Test compression and extraction
    """
    def __init__(self, test_env : Environment):
        super().__init__(
            test_env=test_env,
            test_root=TESTROOT,
            start_dir=TESTROOT,
            test_dirs=[TESTROOT],
            key_inputs=[':', 'mkdir dir1', keys.KEY_ENTER, ':', tconst.FILE_CREATE_COMMAND + ' file1', keys.KEY_ENTER],
            validate_exists=[DIR1, FILE1]
        )
