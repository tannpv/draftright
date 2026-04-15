project('draftright', version: '1.0.0', meson_version: '>= 0.62.0')

python = import('python')
py_installation = python.find_installation('python3')

# Install Python package
install_subdir('draftright', install_dir: py_installation.get_install_dir())

# Install data files
install_data('data/com.draftright.app.desktop', install_dir: get_option('datadir') / 'applications')
install_data('data/com.draftright.app.metainfo.xml', install_dir: get_option('datadir') / 'metainfo')

# Install binary wrapper
install_data('draftright-launch.sh', install_dir: get_option('bindir'), rename: 'draftright')
