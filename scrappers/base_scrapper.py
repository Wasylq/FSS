"asd"

import re
import time
import logging
from random import randrange
from bs4 import BeautifulSoup
from config_parser import CONFIG
from scene import Scene


class BaseScrapper:
    "class"

    base_domain: str = ""
    date_format: str = ""
    suggested_date_format: str = ""
    studio_id: int = None
    scene_link: str = ""
    all_vids: list[type(Scene)] = []
    _requests_headers = {}
    _timeouts = {}

    def __init__(self, url, change_date_format="%Y-%m-%d"):
        pattern = r"(https?://)?(www\d?\.)?(?P<domain>[\w\.-]+\.\w+)(/\S*)?"
        match = re.match(pattern, url)
        if match:
            self.base_domain = match.group("domain")
            logging.debug("The domain has been matched")
        else:
            logging.error("The domain has NOT been matched")
            self.base_domain = None

        self.change_date_format = change_date_format
        self.studio_id = [int(i) for i in url.split("/") if i.isdigit()][0]
        self.studio_link = url
        self._requests_headers["User-Agent"] = CONFIG['connections'].get(
            'user_agent', None)
        self._requests_headers["Referer"] = self.studio_link
        self._timeouts["scenes"] = int(
            CONFIG['connections'].get('timeout_scenes', 60))
        self._timeouts["studio"] = int(
            CONFIG['connections'].get('timeout_studio_pages', 60))

    def parse_html(self, html_as_string) -> BeautifulSoup:
        """Turns HTML code from string to
        BeautifulSoup4 object"""
        logging.debug("Turning html string into bs4 object")
        return BeautifulSoup(html_as_string, "html.parser")

    def get_url(self, vid_html) -> str:
        """Retrieves link to the video
        from <link rel>"""
        parsed_html = self.parse_html(vid_html)
        link = parsed_html.find("link", attrs={"rel": "canonical"})["href"]
        logging.debug("Found link to the scene: %s", link)
        return link

    def to_scenes(self, raw_list) -> list[type(Scene)]:
        """Converts recieved raw list to
        list of Scene objects"""
        vids = []
        for vid in raw_list:
            if int(CONFIG['connections'].get('wait_between_connections', 5)) != 0 and CONFIG['connections'].get('wait_between_connections_random', None):
                sleep = randrange(
                    int(CONFIG['connections'].get('wait_between_connections', 5)))
            elif CONFIG['connections'].get('wait_between_connections', 5):
                sleep = int(CONFIG['connections'].get(
                    'wait_between_connections', 5))
            if CONFIG['connections'].get('wait_between_connections', 5):
                logging.info("waiting %d seconds before next request", sleep)
                time.sleep(sleep)

            logging.debug("Working on vid_ID: %s", vid["id"])
            vid_html = self.get_scene_html(vid["id"])
            vids.append(self.to_scene(vid_html))
        return vids

    def to_scene(self, scene_html):
        """Converts HTML of the scene details 
        to Scene object"""
        title = self.get_title(scene_html)
        logging.info("Processing: %s", title)
        scene = Scene(title)
        logging.debug("Initialized: %s", scene.title)
        logging.debug("Will try to find the date")
        scene.date = self.get_date_released(scene_html)
        scene.dates = self.get_date_released(scene_html)
        logging.debug("Will try to find URL")
        scene.urls = self.get_url(scene_html)
        logging.debug("Will try to find video details")
        scene.details = self.get_details(scene_html)
        logging.debug("Will try to find studio_code")
        scene.studio_code = self.get_studio_code(scene_html)
        logging.debug("Will try to find the duration")
        scene.duration = self.get_duration(scene_html)
        logging.debug("Will try to find the resolution")
        scene.resolutions = self.get_resolution(scene_html)
        logging.debug("Getting info of %s ended", scene.title)
        return scene


    # Parsers should override whats below this line
    def get_all_vids(self):
        """Grabs all scenes from the studio
        and populaes the vids_data attribute"""
        raise NotImplementedError

    def get_scene_html(self, vid_id) -> str:
        """Creates the URL to scene_id and grabs HTML
        to scrape info from"""
        raise NotImplementedError

    def get_title(self, vid_html) -> str:
        "Grabs title from the details of the scene"
        raise NotImplementedError

    def get_date_released(self, vid_html) -> str:
        "Grabs release_date from the details of the scene"
        raise NotImplementedError

    def get_details(self, vid_html) -> str:
        "Grabs scene details from the details of the scene"
        raise NotImplementedError

    def get_studio_code(self, vid_html) -> str:
        "Grabs studio_code from the details of the scene"
        raise NotImplementedError

    def get_duration(self, vid_html) -> str:
        "Grabs duration of the scene from the details of the scene"
        raise NotImplementedError

    def get_resolution(self, vid_html) -> str:
        "Grabs resolution of the scene from the details of the scene"
        raise NotImplementedError
