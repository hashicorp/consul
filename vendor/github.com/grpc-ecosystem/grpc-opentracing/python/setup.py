import os
import logging
import sys
import tempfile

from setuptools import setup, find_packages


def readme():
  """Use `pandoc` to convert `README.md` into a `README.rst` file."""
  if os.path.isfile('README.md') and any('dist' in x for x in sys.argv[1:]):
    if os.system('pandoc -s README.md -o %s/README.rst' %
                 tempfile.mkdtemp()) != 0:
      logging.warning('Unable to generate README.rst')
  if os.path.isfile('README.rst'):
    with open('README.rst') as fd:
      return fd.read()
  return ''


setup(
    name='grpcio-opentracing',
    version='1.0',
    description='Python OpenTracing Extensions for gRPC',
    long_description=readme(),
    author='LightStep',
    license='',
    install_requires=['opentracing>=1.2.2', 'grpcio>=1.1.3', 'six>=1.10'],
    setup_requires=['pytest-runner'],
    tests_require=['pytest', 'future'],
    keywords=['opentracing'],
    classifiers=[
        'Operating System :: OS Independent',
        'Programming Language :: Python :: 2.7',
    ],
    packages=find_packages(exclude=['docs*', 'tests*', 'examples*']))
