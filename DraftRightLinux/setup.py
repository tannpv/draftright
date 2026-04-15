from setuptools import setup, find_packages

setup(
    name="draftright",
    version="1.0.0",
    description="DraftRight — AI-powered text rewriting for Linux",
    author="Tan Nguyen",
    packages=find_packages(),
    python_requires=">=3.10",
    install_requires=[
        "PyGObject>=3.44.0",
        "pycairo>=1.24.0",
        "requests>=2.31.0",
    ],
    entry_points={
        "console_scripts": [
            "draftright=draftright.main:main",
        ],
    },
    package_data={
        "draftright": ["resources/*.css", "resources/*.xml", "resources/icons/*"],
    },
)
