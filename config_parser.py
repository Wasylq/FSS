"Config parser module to avoid cyclical import"
import configparser

CONFIG = configparser.ConfigParser()
CONFIG.read('config.ini')
