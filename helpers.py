"helper functions"
import re
import logging
from datetime import datetime


def remove_multiple_from_string(r_from, list_to_remove, trim_whitespace=True):
    "removes list of strings from string"
    if not list_to_remove:
        return r_from
    tmp = r_from
    for s in list_to_remove:
        tmp = tmp.replace(s, "")
    return tmp


def remove_from_string(r_from, s_remove, trim_whitespace=True):
    """Removes s_remove from r_from"""
    if r_from and s_remove.lower() in r_from.lower():
        s = r_from.replace(s_remove, '')
        if trim_whitespace:
            return " ".join(s.split())
        return s
    logging.debug("Couldn't find %s in %s", s_remove, r_from)
    return r_from


def prepare_scene_key(
        scene_key, chars_to_replace=None,
        replace_with="-", delete_from_title=None
):
    "Prepares scene key that aggregates all realeses"
    s_key = remove_multiple_from_string(
        scene_key,
        ["!", "?", ";", ":", "@", ".", ",",
         " ", "'", '"',
         "classic", "classics", "standard",
         "/", "\\", "&", "(", ")", "*", "%"]
    )
    if not chars_to_replace:
        chars_to_replace = ["'", '"']
    s_key = remove_multiple_from_string(s_key, delete_from_title)
    for char in chars_to_replace:
        s_key = s_key.replace(char, replace_with)
    s_key = s_key.lower()
    return s_key.strip()


def remove_regex(text, regex, trim_whitespace=True):
    "Removes regex from string"
    match = re.search(rf"{regex}", text)
    if match:
        return remove_from_string(
            text, match.group(),
            trim_whitespace=trim_whitespace
        )
    return None


def remove_parenthesis(text, trim_whitespace=True):
    "Removes text inside parenthesis from string"
    pattern = r"\(.*\)"
    return remove_regex(text, regex=pattern, trim_whitespace=trim_whitespace)


def merge_equal_videos(list_of_scenes, delete_from_title=None):
    """Iterates through the list of scenes and merges them
    based on the title, in the process finding all URLs to
    different releases of the same scene, and gets the earliest
    release date"""
    merged_list = {}
    for scene in list_of_scenes:
        logging.info("working on scene: %s", scene.title)
        k_title = prepare_scene_key(
            scene.title, delete_from_title=delete_from_title)
        logging.info("scene key is now: %s", k_title)
        if merged_list.get(k_title):
            logging.info("found scene key %s in the dict", k_title)
            existing = merged_list[k_title]
            decide_values_for_merged_scene(existing, scene, delete_from_title)
        else:
            logging.info("not found: %s in list, adding it", k_title)
            title = remove_multiple_from_string(scene.title, delete_from_title)
            scene.title = title.strip()
            scene.earliest_date = scene.date
            merged_list[k_title] = scene
    return merged_list


def decide_values_for_merged_scene(existing, current, delete_from_title=None):
    "Overwrite the values of existing scene in the dictionary"
    logging.info("old scene is: %s, %s, %s, %s",
                 existing.title, existing.date,
                 existing.urls, existing.resolutions
                 )
    existing.resolutions = current.resolutions
    existing.urls = current.urls
    title = remove_multiple_from_string(current.title, delete_from_title)
    existing.title = title.strip()
    existing.dates = current.date
    logging.info("existing scene is: %s, %s, %s, %s",
                 existing.title, existing.date,
                 existing.urls, existing.resolutions
                 )
    scene_date = datetime.strptime(current.date, current.date_format)
    old_scene_date = datetime.strptime(existing.date, existing.date_format)
    if old_scene_date > scene_date:
        existing.earliest_date = current.date
    else:
        existing.earliest_date = current.date
    existing.date = current.date
