"""
Clips4Sale scrapper designed to search for the same videos based on the title,
find the earliest date of release and all links to the same scene
"""
import logging
from datetime import datetime
import requests
from .base_scrapper import BaseScrapper


class C4SScrapper (BaseScrapper):
    """
    Clips4Sale scrapper designed to search for the same videos based on the title,
    find the earliest date of release and all links to the same scene
    """
    suggested_date_format = "%m/%d/%y"  # 4/31/24
    json_location = "{url}/Cat0-AllCategories/Page{page_no}/C4SSort-added_at/Limit24/?onlyClips=true&_data=routes%2F($lang).studio.$id_.$studioSlug.$"
    # json_location = "https://www.{domain}/en/studio/{studio_id}/{studio_name}/Cat0-AllCategories/Page{page_no}/C4SSort-added_at/Limit24/?onlyClips=true&_data=routes%2F($lang).studio.$id_.$studioSlug.$"
    # scene_link = "https://www.{domain}/studio/{studio_id}/{scene_id}/"
    scene_link = "https://www.{domain}/studio/{studio_id}/{scene_id}/"

    def get_all_vids(self):
        """Grabs all scenes from the studio
        and populaes the vids_data attribute"""
        logging.info("Grabbing all vids from %s", self.studio_link)
        page_number = 1
        logging.info("working on page %d", page_number)
        r = requests.get(
            self.json_location.format(
                url=self.studio_link, page_no=page_number),
            timeout=self._timeouts["studio"]
        )
        vids_data = r.json()['clips']
        while vids_data != []:
            self.all_vids.extend(self.to_scenes(vids_data))
            page_number += 1
            logging.info("working on page %d", page_number)
            r = requests.get(
                self.json_location.format(
                    url=self.studio_link, page_no=page_number),
                timeout=self._timeouts["scenes"],
                headers=self._requests_headers
            )
            vids_data = r.json().get('clips', [])
        logging.info("Finished processing all vids from %s", self.studio_link)

    def get_scene_html(self, vid_id) -> str:
        """Creates the URL to scene_id and grabs HTML
        to scrape info from"""
        r = requests.get(
            self.scene_link.format(
                domain=self.base_domain, studio_id=self.studio_id, scene_id=vid_id),
            timeout=30,
            headers=self._requests_headers
        )
        if r.status_code != 200:
            logging.debug("Recived %d. Dumping HTML next", r.status_code)
            logging.debug(r.text)
            raise requests.HTTPError
        return r.text

    def get_title(self, vid_html) -> str:
        parsed_html = super().parse_html(vid_html)
        h1 = parsed_html.find("h1")
        title = h1.text
        logging.debug("Found title, removing whitespaces")
        return " ".join(title.split())

    def get_date_released(self, vid_html) -> str:
        parsed_html = super().parse_html(vid_html)
        date_div = parsed_html.find("div", class_="lg:border-0")
        date = date_div.find("span").text.split()[0]
        if self.change_date_format == "" or self.change_date_format is None:
            pass
        else:
            try:
                changed_date = datetime.strptime(
                    date,
                    self.suggested_date_format
                ).strftime(self.change_date_format)
                logging.debug("Found the date")
                return changed_date
            except Exception as e:
                raise e
        return date

    def get_details(self, vid_html) -> str:
        parsed_html = super().parse_html(vid_html)
        descr = parsed_html.find("div", class_="read-more--text").text
        return '\n'.join(' '.join(line.split()) for line in descr.split('\n'))

    def get_studio_code(self, vid_html) -> str:
        # parsed_html = super().parse_html(vid_html)
        return None

    def get_duration(self, vid_html) -> str:
        parsed_html = super().parse_html(vid_html)
        date_div = parsed_html.find("div", class_="lg:border-0")
        duration = date_div.find_all("span")[2].text
        return duration

    def get_resolution(self, vid_html) -> str:
        parsed_html = super().parse_html(vid_html)
        date_div = parsed_html.find("div", class_="lg:border-0")
        resolution = date_div.find_all("span")[8].text
        if resolution == "1080p":
            resolution = "FHD"
        elif resolution == "4k":
            resolution = "4K"
        return resolution
