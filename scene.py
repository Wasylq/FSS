"scene class"

import csv
import sys
import logging


class Scene:
    "Scene"
    title: str
    thumb_url: str
    date_format: str
    _urls: str = None
    _resolutions = []
    _dates: str = None
    date_added: str = ""
    earliest_date: str = ""

    def __init__(self, title, date_format="%Y-%m-%d"):
        self.title = title
        self.date_format = date_format
        self.thumb_url = None
        self._resolutions = []
        self.earliest_date = ""

    @property
    def urls(self):
        "URLs of the scene"
        return self._urls

    @urls.setter
    def urls(self, value):
        if self._urls is None and isinstance(value, str):
            logging.debug("Setting URL to %s", value)
            self._urls = value
        elif isinstance(self._urls, str):
            logging.debug("URL already present, morphing it to list")
            tmp = self._urls
            self._urls = []
            self._urls.append(tmp)
        if isinstance(self._urls, list):
            logging.debug("Adding URL to list of URLs")
            self._urls.append(value)

    @property
    def dates(self):
        "URLs of the scene"
        return self._dates

    @dates.setter
    def dates(self, value):
        if self._dates is None and isinstance(value, str):
            logging.debug("Setting date to %s", value)
            self._dates = value
        elif isinstance(self._dates, str):
            logging.debug("dae already present, morphing it to list")
            tmp = self._dates
            self._dates = []
            self._dates.append(tmp)
        if isinstance(self._dates, list):
            logging.debug("Adding date to list of URLs")
            self._dates.append(value)

    @property
    def resolutions(self):
        "URLs of the scene"
        return self._resolutions

    @resolutions.setter
    def resolutions(self, value):
        if not self._resolutions:
            self._resolutions.append(value)
        else:
            self._resolutions.extend(value)

    def output_csv(self, separator=";", output="stdout", show_headers=False):
        "Output class as CSV file"
        a = vars(self)
        if output == "stdout":
            writer = csv.DictWriter(
                sys.stdout,
                fieldnames=a.keys(),
                delimiter=separator
            )
            if show_headers:
                writer.writeheader()
            writer.writerow(a)
        else:
            with open(output, 'a', newline='') as f:
                writer = csv.DictWriter(
                    f,
                    fieldnames=a.keys(),
                    delimiter=separator
                )
                if show_headers:
                    writer.writeheader()
                writer.writerow(a)
