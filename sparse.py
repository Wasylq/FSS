#!/usr/bin/env python3
"Main file of the script"

import argparse
import logging
import helpers
from config_parser import CONFIG
from scrappers.c4s import C4SScrapper
from scrappers.manyvids import ManyVidsScrapper

parser = argparse.ArgumentParser(
    prog='SParse',
    description='''Parses whole studios to get all videos and
    grab the earliest date and all links for scenes''',
)
parser.add_argument('url', nargs='+',
                    help='Space separated list of studios\
                         to scrape. Ideally all studios of the same performer')
args = parser.parse_args()

level = CONFIG['DEFAULT'].get('log_level', 'INFO')
if level.lower() == "DEBUG".lower():
    LOG_LEVEL = logging.DEBUG
elif level.lower() == "INFO".lower():
    LOG_LEVEL = logging.INFO
elif level.lower() == "WARNING".lower():
    LOG_LEVEL = logging.WARNING
elif level.lower() == "ERROR".lower():
    LOG_LEVEL = logging.ERROR
elif level.lower() == "CRITICAL".lower():
    LOG_LEVEL = logging.CRITICAL

logging.basicConfig(
    level=LOG_LEVEL,
    format='%(asctime)s - %(levelname)s: %(message)s',
    filename=CONFIG['DEFAULT'].get('log_location', 'sparse.log'),
    filemode='a'
)

studio_parsers = []
for studio_url in args.url:
    if "clips4sale.com" in studio_url:
        studio_parsers.append(C4SScrapper(studio_url))
    if "manyvids.com" in studio_url:
        studio_parsers.append(ManyVidsScrapper(studio_url))

vids = []
for parser in studio_parsers:
    parser.get_all_vids()
    vids.extend(parser.all_vids)

merged = helpers.merge_equal_videos(
    parser.all_vids,
    delete_from_title=[
        "Classics", "Classic", "Standard",
        "*", "-", "'", '"', "!",
        "NEW CONTENT", "NEW!", "NEW",
        "(FULL HD)", "(full hd)",
        "4k", "4K", "hd", "HD",
        "step-", "Step-", "step", "Step",
        "wmv", "mp4", "WMV", "MP4",
        "mov", "avi", "(avi)", "()",
    ]
)

for index, scene in enumerate(merged):
    if index == 0:
        merged[scene].output_csv(
            output=CONFIG['DEFAULT'].get('csv_output_location', 'sparse.log'),
            show_headers=True
        )
    else:
        merged[scene].output_csv(
            output=CONFIG['DEFAULT'].get('csv_output_location', 'sparse.log')
        )
